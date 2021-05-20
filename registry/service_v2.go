package registry // import "github.com/docker/docker/registry"

import (
	"github.com/sirupsen/logrus"
	"net/url"
	"strings"

	"github.com/docker/go-connections/tlsconfig"
)

func (s *DefaultService) lookupMirrorRegistryEndpoints(hostname string) []APIEndpoint {
	var (
		regMirror = s.lookupMirrorRegistry(hostname)
		endpoints []APIEndpoint
	)
	if regMirror == nil {
		return endpoints
	}

	rURL := hostname
	if !strings.HasPrefix(rURL, "http://") && !strings.HasPrefix(rURL, "https://") {
		rURL = "https://" + rURL
	}
	rurl, err := url.Parse(rURL)
	if err != nil {
		logrus.Warnf("failed to parse url: %s, err: %s", hostname, err)
		return nil
	}
	if rurl.Host == "" {
		logrus.Warnf("host is empty for hostname: %s", hostname)
		return nil
	}
	// add mirrors
	for _, mirror := range regMirror.Mirrors {
		// if the host same as mirror host, do not have to go to the mirrors
		if mirror.Hostname() == "localhost" || mirror.Hostname() == "127.0.0.1" || mirror.Host == hostname {
			return nil
		}

		mirrorTlsCfg, err := s.tlsConfigForMirror(&mirror)
		if err != nil {
			logrus.Warnf("failed to config mirror %s, err: %s", mirror.String(), err)
			continue
		}
		endpoints = append(endpoints, APIEndpoint{
			URL:          &mirror,
			Version:      APIVersion2,
			TrimHostname: true,
			Mirror:       true,
			TLSConfig:    mirrorTlsCfg,
		})
	}
	// return it directly, leave it to the original docker logic
	if len(endpoints) == 0 {
		return endpoints
	}

	if hostname == DefaultNamespace || hostname == IndexHostname {
		endpoints = append(endpoints, APIEndpoint{
			URL:          DefaultV2Registry,
			Version:      APIVersion2,
			TrimHostname: true,
			Official: true,
			TLSConfig:    tlsconfig.ServerDefault(),
		})
		return endpoints
	}

	normalEndpoints, err := s.setUpNormalEndpoints(hostname)
	if err != nil {
		return nil
	}
	return append(endpoints, normalEndpoints...)
}

// copy from lookupV2Endpoints, aims to setup the registry endpoints except dockerhub
func (s *DefaultService) setUpNormalEndpoints(hostname string) ([]APIEndpoint, error) {
	var (
		ana = allowNondistributableArtifacts(s.config, hostname)
		endpoints []APIEndpoint
	)

	tlsConfig, err := s.tlsConfig(hostname)
	if err != nil {
		return nil, err
	}

	endpoints = []APIEndpoint{
		{
			URL: &url.URL{
				Scheme: "https",
				Host:   hostname,
			},
			Version:                        APIVersion2,
			AllowNondistributableArtifacts: ana,
			TrimHostname:                   true,
			TLSConfig:                      tlsConfig,
		},
	}

	if tlsConfig.InsecureSkipVerify {
		endpoints = append(endpoints, APIEndpoint{
			URL: &url.URL{
				Scheme: "http",
				Host:   hostname,
			},
			Version:                        APIVersion2,
			AllowNondistributableArtifacts: ana,
			TrimHostname:                   true,
			// used to check if supposed to be secure via InsecureSkipVerify
			TLSConfig: tlsConfig,
		})
	}

	return endpoints, nil
}

func (s *DefaultService) lookupV2Endpoints(hostname string) (endpoints []APIEndpoint, err error) {
	tlsConfig := tlsconfig.ServerDefault()

	endpoints = s.lookupMirrorRegistryEndpoints(hostname)
	if len(endpoints) > 0 {
		return endpoints, nil
	}

	if hostname == DefaultNamespace || hostname == IndexHostname {
		// v2 mirrors
		for _, mirror := range s.config.Mirrors {
			if !strings.HasPrefix(mirror, "http://") && !strings.HasPrefix(mirror, "https://") {
				mirror = "https://" + mirror
			}
			mirrorURL, err := url.Parse(mirror)
			if err != nil {
				return nil, err
			}
			mirrorTLSConfig, err := s.tlsConfigForMirror(mirrorURL)
			if err != nil {
				return nil, err
			}
			endpoints = append(endpoints, APIEndpoint{
				URL: mirrorURL,
				// guess mirrors are v2
				Version:      APIVersion2,
				Mirror:       true,
				TrimHostname: true,
				TLSConfig:    mirrorTLSConfig,
			})
		}
		// v2 registry
		endpoints = append(endpoints, APIEndpoint{
			URL:          DefaultV2Registry,
			Version:      APIVersion2,
			Official:     true,
			TrimHostname: true,
			TLSConfig:    tlsConfig,
		})

		return endpoints, nil
	}

	ana := allowNondistributableArtifacts(s.config, hostname)

	tlsConfig, err = s.tlsConfig(hostname)
	if err != nil {
		return nil, err
	}

	endpoints = []APIEndpoint{
		{
			URL: &url.URL{
				Scheme: "https",
				Host:   hostname,
			},
			Version:                        APIVersion2,
			AllowNondistributableArtifacts: ana,
			TrimHostname:                   true,
			TLSConfig:                      tlsConfig,
		},
	}

	if tlsConfig.InsecureSkipVerify {
		endpoints = append(endpoints, APIEndpoint{
			URL: &url.URL{
				Scheme: "http",
				Host:   hostname,
			},
			Version:                        APIVersion2,
			AllowNondistributableArtifacts: ana,
			TrimHostname:                   true,
			// used to check if supposed to be secure via InsecureSkipVerify
			TLSConfig: tlsConfig,
		})
	}

	return endpoints, nil
}
