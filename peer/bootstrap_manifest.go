package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type BootstrapPeerStatus struct {
	ID                string  `json:"id"`
	Label             string  `json:"label"`
	Host              string  `json:"host"`
	Port              int     `json:"port"`
	Status            string  `json:"status"`
	AgeDays           int     `json:"ageDays"`
	ExternalObservers int     `json:"externalObservers"`
	Grandfathered     bool    `json:"grandfathered"`
	VettingUptime     float64 `json:"vettingUptimeRatio"`
	UptimeRatio365d   float64 `json:"uptimeRatio365d"`
}

type BootstrapRegistryStatus struct {
	ManifestURL   string                `json:"manifestUrl"`
	LastRefreshAt string                `json:"lastRefreshAt"`
	LastError     string                `json:"lastError"`
	Policy        BootstrapPolicy       `json:"policy"`
	Peers         []BootstrapPeerStatus `json:"peers"`
	Candidates    []BootstrapPeerStatus `json:"candidates"`
	FallbackPeers []string              `json:"fallbackPeers"`
}

type BootstrapPolicy struct {
	VettingDays              int     `json:"vettingDays"`
	VettingMinimumUptime     float64 `json:"vettingMinimumUptimeRatio"`
	BootstrapMaturityDays    int     `json:"bootstrapMaturityDays"`
	EligibilityWindowDays    int     `json:"eligibilityWindowDays"`
	MinimumUptimeRatio       float64 `json:"minimumUptimeRatio"`
	MinimumExternalObservers int     `json:"minimumExternalObservers"`
}

type bootstrapManifest struct {
	SchemaVersion int                   `json:"schemaVersion"`
	GeneratedAt   string                `json:"generatedAt"`
	ValidForHours int                   `json:"validForHours"`
	Policy        BootstrapPolicy       `json:"policy"`
	Peers         []BootstrapPeerStatus `json:"peers"`
	Candidates    []BootstrapPeerStatus `json:"candidates"`
	FallbackPeers []string              `json:"fallbackPeers"`
}

type BootstrapRegistry struct {
	url             string
	refreshInterval time.Duration
	client          *http.Client
	mu              sync.RWMutex
	status          BootstrapRegistryStatus
}

func NewBootstrapRegistry(url string, refreshHours int, fallback []PeerAddress) *BootstrapRegistry {
	if refreshHours <= 0 {
		refreshHours = 24
	}
	return &BootstrapRegistry{
		url:             url,
		refreshInterval: time.Duration(refreshHours) * time.Hour,
		client:          &http.Client{Timeout: 8 * time.Second},
		status: BootstrapRegistryStatus{
			ManifestURL:   url,
			FallbackPeers: peerAddressStrings(fallback),
		},
	}
}

func (r *BootstrapRegistry) Peers(ctx context.Context) []PeerAddress {
	peers, err := r.fetch(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.status.LastError = err.Error()
		return nil
	}
	r.status.LastError = ""
	r.status.LastRefreshAt = time.Now().UTC().Format(time.RFC3339)
	r.status.Peers = peers
	return bootstrapStatusesToAddresses(peers)
}

func (r *BootstrapRegistry) CachedPeers() []PeerAddress {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return bootstrapStatusesToAddresses(r.status.Peers)
}

func (r *BootstrapRegistry) Status() BootstrapRegistryStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := r.status
	out.Peers = append([]BootstrapPeerStatus(nil), r.status.Peers...)
	out.Candidates = append([]BootstrapPeerStatus(nil), r.status.Candidates...)
	out.FallbackPeers = append([]string(nil), r.status.FallbackPeers...)
	return out
}

func (r *BootstrapRegistry) Start(ctx context.Context) {
	if r == nil || r.url == "" {
		return
	}
	ticker := time.NewTicker(r.refreshInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = r.Peers(ctx)
			}
		}
	}()
}

func (r *BootstrapRegistry) fetch(ctx context.Context) ([]BootstrapPeerStatus, error) {
	if r == nil || r.url == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("manifest returned HTTP %d", resp.StatusCode)
	}
	var manifest bootstrapManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.status.Policy = manifest.Policy
	r.status.Candidates = append([]BootstrapPeerStatus(nil), manifest.Candidates...)
	r.status.FallbackPeers = append([]string(nil), manifest.FallbackPeers...)
	r.mu.Unlock()
	return manifest.Peers, nil
}

func mergeBootstrapPeers(groups ...[]PeerAddress) []PeerAddress {
	seen := make(map[string]PeerAddress)
	for _, group := range groups {
		for _, address := range group {
			if address.Host == "" || address.Port <= 0 {
				continue
			}
			if address.ID == "" {
				address.ID = strconv.Itoa(address.Port)
			}
			seen[address.Host+":"+strconv.Itoa(address.Port)] = address
		}
	}
	out := make([]PeerAddress, 0, len(seen))
	for _, address := range seen {
		out = append(out, address)
	}
	return out
}

func bootstrapStatusesToAddresses(peers []BootstrapPeerStatus) []PeerAddress {
	out := make([]PeerAddress, 0, len(peers))
	for _, peer := range peers {
		if peer.Host == "" || peer.Port <= 0 {
			continue
		}
		id := peer.ID
		if id == "" {
			id = strconv.Itoa(peer.Port)
		}
		out = append(out, PeerAddress{ID: id, Host: peer.Host, Port: peer.Port})
	}
	return out
}

func peerAddressStrings(addresses []PeerAddress) []string {
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address.Host != "" && address.Port > 0 {
			out = append(out, address.Host+":"+strconv.Itoa(address.Port))
		}
	}
	return out
}
