//+kubebuilder:object:generate=true
package headscale

import (
	"crypto/tls"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	ServerURL                      string          `json:"server_url,omitempty"`
	Addr                           string          `json:"listen_addr,omitempty"`
	MetricsAddr                    string          `json:"metrics_listen_addr,omitempty"`
	GRPCAddr                       string          `json:"grpc_listen_addr,omitempty"`
	GRPCAllowInsecure              *bool           `json:"grpc_allow_insecure,omitempty"`
	EphemeralNodeInactivityTimeout metav1.Duration `json:"ephemeral_node_inactivity_timeout,omitempty"`
	IPPrefixes                     []string        `json:"ip_prefixes,omitempty"`
	PrivateKeyPath                 string          `json:"private_key_path,omitempty"`

	DERP DERPConfig `json:"derp,omitempty"`

	DBtype string `json:"db_type,omitempty"`
	DBpath string `json:"db_path,omitempty"`
	DBhost string `json:"db_host,omitempty"`
	DBport int    `json:"db_port,omitempty"`
	DBname string `json:"db_name,omitempty"`
	DBuser string `json:"db_user,omitempty"`
	DBpass string `json:"db_pass,omitempty"`

	TLSLetsEncryptListen        string `json:"tls_letsencrypt_listen,omitempty"`
	TLSLetsEncryptHostname      string `json:"tls_letsencrypt_hostname,omitempty"`
	TLSLetsEncryptCacheDir      string `json:"tls_letsencrypt_cache_dir,omitempty"`
	TLSLetsEncryptChallengeType string `json:"tls_letsencrypt_challenge_type,omitempty"`

	TLSCertPath       string             `json:"tls_cert_path,omitempty"`
	TLSKeyPath        string             `json:"tls_key_path,omitempty"`
	TLSClientAuthMode tls.ClientAuthType `json:"tls_client_auth_mode,omitempty"`

	ACMEURL   string `json:"acme_url,omitempty"`
	ACMEEmail string `json:"acme_email,omitempty"`

	DNSConfig DNSConfig `json:"dns_config,omitempty"`

	UnixSocket           string `json:"unix_socket,omitempty"`
	UnixSocketPermission string `json:"unix_socket_permission,omitempty"`

	OIDC OIDCConfig `json:"oidc,omitempty"`

	LogTail LogTailConfig `json:"logtail,omitempty"`

	LogLevel string `json:"log_level,omitempty"`

	CLI CLIConfig `json:"cli,omitempty"`
}

type OIDCConfig struct {
	Issuer           string            `json:"issuer,omitempty"`
	ClientID         string            `json:"client_id,omitempty"`
	ClientSecret     string            `json:"client_secret,omitempty"`
	Scope            []string          `json:"scope,omitempty"`
	ExtraParams      map[string]string `json:"extra_params,omitempty"`
	AllowedDomains   []string          `json:"allowed_domains,omitempty"`
	AllowedUsers     []string          `json:"allowed_users,omitempty"`
	StripEmaildomain *bool             `json:"strip_email_domain,omitempty"`
}

type DERPConfig struct {
	Server          DERPConfigServer `json:"server,omitempty"`
	AutoUpdate      *bool            `json:"auto_update_enabled,omitempty"`
	URLs            []string         `json:"urls,omitempty"`
	Paths           []string         `json:"paths,omitempty"`
	UpdateFrequency metav1.Duration  `json:"update_frequency,omitempty"`
}

type DNSConfig struct {
	Magic       *bool    `json:"magic_dns,omitempty"`
	BaseDomain  string   `json:"base_domain,omitempty"`
	Nameservers []string `json:"nameservers,omitempty"`
	Domains     []string `json:"domains,omitempty"`
}

type DERPConfigServer struct {
	Enabled    *bool  `json:"enabled,omitempty"`
	RegionCode string `json:"region_code,omitempty"`
	RegionName string `json:"region_name,omitempty"`
	STUNAddr   string `json:"stun_listen_addr,omitempty"`
	RegionID   int    `json:"region_id,omitempty"`
}

type LogTailConfig struct {
	Enabled *bool `json:"enable,omitempty"`
}

type CLIConfig struct {
	Insecure *bool           `json:"insecure,omitempty"`
	Address  string          `json:"address,omitempty"`
	APIKey   string          `json:"api_key,omitempty"`
	Timeout  metav1.Duration `json:"timeout,omitempty"`
}
