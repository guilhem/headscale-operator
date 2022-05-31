package controllers

import (
	"time"

	"github.com/guilhem/headscale-operator/pkg/headscale"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var defaultServerConfig = headscale.Config{
	Addr:        "0.0.0.0:8080",
	MetricsAddr: "0.0.0.0:8081",
	// GRPCAddr:    "0.0.0.0:8081",
	DERP: headscale.DERPConfig{
		Server: headscale.DERPConfigServer{
			Enabled:    pointer.Bool(false),
			RegionID:   999,
			RegionCode: "headscale",
			RegionName: "Headscale Embedded DERP",
			STUNAddr:   "0.0.0.0:3478",
		},
		URLs:            []string{"https://controlplane.tailscale.com/derpmap/default"},
		AutoUpdate:      pointer.Bool(true),
		Paths:           []string{},
		UpdateFrequency: metav1.Duration{Duration: time.Hour * 1},
	},
	EphemeralNodeInactivityTimeout: metav1.Duration{Duration: time.Hour * 24},
	// ACMEURL:                        "https://acme-v02.api.letsencrypt.org/directory",
	// ACMEEmail:                      "",
	DNSConfig: headscale.DNSConfig{
		Nameservers: []string{"1.1.1.1"},
		Magic:       pointer.Bool(true),
		Domains:     []string{},
		BaseDomain:  "",
	},
	LogLevel: "info",
}
