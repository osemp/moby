package registry // import "github.com/docker/docker/api/types/registry"

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/opencontainers/image-spec/specs-go/v1"
)

type regMirrorJSONHelper struct {
	Domain  string   `json:"domain,omitempty"` // domain is domainName:port(if port is specified)
	Mirrors []string `json:"mirrors,omitempty"`
}

type RegMirror struct {
	Domain  url.URL   `json:"domain,omitempty"` // domain is domainName:port(if port is specified)
	Mirrors []url.URL `json:"mirrors,omitempty"`
}

func (rm *RegMirror) UnmarshalJSON(data []byte) error {
	var (
		helper     = regMirrorJSONHelper{}
		domainURL  url.URL
		mirrorURLs []url.URL
	)

	err := json.Unmarshal(data, &helper)
	if err != nil {
		return err
	}

	u, err := parseURL(helper.Domain)
	if err != nil {
		return err
	}
	domainURL = *u

	for _, m := range helper.Mirrors {
		u, err = parseURL(m)
		if err != nil {
			return err
		}
		mirrorURLs = append(mirrorURLs, *u)
	}

	rm.Domain, rm.Mirrors = domainURL, mirrorURLs
	return nil
}

func (rm *RegMirror) MarshalJSON() ([]byte, error) {
	var (
		helper     = regMirrorJSONHelper{}
		domainURL  string
		mirrorURLs []string
	)

	domainURL = rm.Domain.String()
	for _, mirror := range rm.Mirrors {
		mirrorURLs = append(mirrorURLs, mirror.String())
	}
	helper.Domain, helper.Mirrors = domainURL, mirrorURLs
	return json.Marshal(helper)
}

func (rm *RegMirror) ContainerMirror(str string) bool {
	for _, mirror := range rm.Mirrors {
		if mirror.String() == str {
			return true
		}
	}
	return false
}

func NewRegistryMirror(domain string, mirrors []string) (RegMirror, error) {
	reg := RegMirror{}
	domainU, err := parseURL(domain)
	if err != nil {
		return RegMirror{}, err
	}

	reg.Domain = *domainU
	for _, str := range mirrors {
		mirrorU, err := parseURL(str)
		if err != nil {
			return RegMirror{}, err
		}
		reg.Mirrors = append(reg.Mirrors, *mirrorU)
	}
	return reg, nil
}

// ServiceConfig stores daemon registry services configuration.
type ServiceConfig struct {
	AllowNondistributableArtifactsCIDRs     []*NetIPNet
	AllowNondistributableArtifactsHostnames []string
	InsecureRegistryCIDRs                   []*NetIPNet           `json:"InsecureRegistryCIDRs"`
	IndexConfigs                            map[string]*IndexInfo `json:"IndexConfigs"`
	Mirrors                                 []string
	RegMirrors                              map[string]RegMirror
}

// NetIPNet is the net.IPNet type, which can be marshalled and
// unmarshalled to JSON
type NetIPNet net.IPNet

// String returns the CIDR notation of ipnet
func (ipnet *NetIPNet) String() string {
	return (*net.IPNet)(ipnet).String()
}

// MarshalJSON returns the JSON representation of the IPNet
func (ipnet *NetIPNet) MarshalJSON() ([]byte, error) {
	return json.Marshal((*net.IPNet)(ipnet).String())
}

// UnmarshalJSON sets the IPNet from a byte array of JSON
func (ipnet *NetIPNet) UnmarshalJSON(b []byte) (err error) {
	var ipnetStr string
	if err = json.Unmarshal(b, &ipnetStr); err == nil {
		var cidr *net.IPNet
		if _, cidr, err = net.ParseCIDR(ipnetStr); err == nil {
			*ipnet = NetIPNet(*cidr)
		}
	}
	return
}

// IndexInfo contains information about a registry
//
// RepositoryInfo Examples:
// {
//   "Index" : {
//     "Name" : "docker.io",
//     "Mirrors" : ["https://registry-2.docker.io/v1/", "https://registry-3.docker.io/v1/"],
//     "Secure" : true,
//     "Official" : true,
//   },
//   "RemoteName" : "library/debian",
//   "LocalName" : "debian",
//   "CanonicalName" : "docker.io/debian"
//   "Official" : true,
// }
//
// {
//   "Index" : {
//     "Name" : "127.0.0.1:5000",
//     "Mirrors" : [],
//     "Secure" : false,
//     "Official" : false,
//   },
//   "RemoteName" : "user/repo",
//   "LocalName" : "127.0.0.1:5000/user/repo",
//   "CanonicalName" : "127.0.0.1:5000/user/repo",
//   "Official" : false,
// }
type IndexInfo struct {
	// Name is the name of the registry, such as "docker.io"
	Name string
	// Mirrors is a list of mirrors, expressed as URIs
	Mirrors []string
	// Secure is set to false if the registry is part of the list of
	// insecure registries. Insecure registries accept HTTP and/or accept
	// HTTPS with certificates from unknown CAs.
	Secure bool
	// Official indicates whether this is an official registry
	Official bool
}

// SearchResult describes a search result returned from a registry
type SearchResult struct {
	// StarCount indicates the number of stars this repository has
	StarCount int `json:"star_count"`
	// IsOfficial is true if the result is from an official repository.
	IsOfficial bool `json:"is_official"`
	// Name is the name of the repository
	Name string `json:"name"`
	// IsAutomated indicates whether the result is automated
	IsAutomated bool `json:"is_automated"`
	// Description is a textual description of the repository
	Description string `json:"description"`
}

// SearchResults lists a collection search results returned from a registry
type SearchResults struct {
	// Query contains the query string that generated the search results
	Query string `json:"query"`
	// NumResults indicates the number of results the query returned
	NumResults int `json:"num_results"`
	// Results is a slice containing the actual results for the search
	Results []SearchResult `json:"results"`
}

// DistributionInspect describes the result obtained from contacting the
// registry to retrieve image metadata
type DistributionInspect struct {
	// Descriptor contains information about the manifest, including
	// the content addressable digest
	Descriptor v1.Descriptor
	// Platforms contains the list of platforms supported by the image,
	// obtained by parsing the manifest
	Platforms []v1.Platform
}

func parseURL(str string) (*url.URL, error) {
	str = strings.ToLower(str)
	newURL, err := url.Parse(str)
	if err != nil {
		return nil, err
	}
	if newURL.Scheme == "" {
		newURL.Scheme = "https"
		return parseURL("https://" + str)
	}
	if newURL.Host == "" {
		return nil, fmt.Errorf("failed to parse %s to url, err: host is empty", str)
	}
	if newURL.Scheme != "http" && newURL.Scheme != "https" {
		return nil, fmt.Errorf("failed to parse %s to url, err: unsupported scheme %s", str, newURL.Scheme)
	}

	return newURL, nil
}
