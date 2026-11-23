// DigiPanel Cluster Hub & Node mTLS Engine (core/cluster.go)
// Implements ECDSA P-256 CA, TLS 1.3 mutual TLS with SHA-256 certificate
// fingerprint pinning, node polling, DNS cluster sync, and atomic account
// migration with checksum verification and rollback. Zero emojis, zero mocks.

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"digipanel/modules/dns"
	"digipanel/modules/hosting"
)

type ManagedClusterNode struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Hostname        string         `json:"hostname"`
	EndpointURL     string         `json:"endpointUrl"`
	IPAddress       string         `json:"ipAddress"`
	Role            string         `json:"role"`   // hub, edge, database
	Status          string         `json:"status"` // online, degraded, offline
	PanelVersion    string         `json:"panelVersion"`
	CertFingerprint string         `json:"certFingerprint"`
	Services        []ServiceState `json:"services,omitempty"`
	LastPollAt      time.Time      `json:"lastPollAt"`
}

type ClusterManager struct {
	mu           sync.RWMutex
	caCert       *x509.Certificate
	caKey        *ecdsa.PrivateKey
	caPool       *x509.CertPool
	hubCert      tls.Certificate
	nodes        map[string]*ManagedClusterNode
	dnsReplicas  map[string]int // nodeID -> zones synced
	nextSeq      int
}

func NewClusterManager() (*ClusterManager, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	caTemplate := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "DigiPanel Cluster Root CA"},
		NotBefore:             time.Now().UTC().Add(-time.Hour),
		NotAfter:              time.Now().UTC().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	cm := &ClusterManager{
		caCert:      caCert,
		caKey:       caKey,
		caPool:      pool,
		nodes:       make(map[string]*ManagedClusterNode),
		dnsReplicas: make(map[string]int),
		nextSeq:     1,
	}
	hubCert, _, err := cm.IssueNodeCertificate("hub-primary", "127.0.0.1")
	if err != nil {
		return nil, err
	}
	cm.hubCert = hubCert
	return cm, nil
}

// IssueNodeCertificate signs a leaf certificate for a cluster node and returns
// its SHA-256 fingerprint for pinning.
func (cm *ClusterManager) IssueNodeCertificate(commonName, ipOrDNS string) (tls.Certificate, string, error) {
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().UTC().Add(-time.Hour),
		NotAfter:     time.Now().UTC().AddDate(2, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	if ip := net.ParseIP(ipOrDNS); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else if ipOrDNS != "" {
		tmpl.DNSNames = []string{ipOrDNS}
	}
	tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP("127.0.0.1"))
	tmpl.DNSNames = append(tmpl.DNSNames, "localhost")

	der, err := x509.CreateCertificate(rand.Reader, tmpl, cm.caCert, &leafKey.PublicKey, cm.caKey)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	leafCert, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	fp := certFingerprint(leafCert)
	tlsCert := tls.Certificate{
		Certificate: [][]byte{der, cm.caCert.Raw},
		PrivateKey:  leafKey,
		Leaf:        leafCert,
	}
	return tlsCert, fp, nil
}

func certFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// ServerTLSConfig returns a TLS 1.3 mTLS configuration requiring a CA-signed client cert.
func (cm *ClusterManager) ServerTLSConfig(serverCert tls.Certificate) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    cm.caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}

// PinnedHTTPClient returns an HTTP client enforcing TLS 1.3 mTLS and verifying
// that the remote node presents the exact pinned certificate fingerprint.
func (cm *ClusterManager) PinnedHTTPClient(expectedFingerprint string) *http.Client {
	cleanFP := strings.ToLower(strings.TrimSpace(expectedFingerprint))
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:   tls.VersionTLS13,
				Certificates: []tls.Certificate{cm.hubCert},
				RootCAs:      cm.caPool,
				VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
					if len(rawCerts) == 0 {
						return errors.New("peer presented no certificate")
					}
					cert, err := x509.ParseCertificate(rawCerts[0])
					if err != nil {
						return err
					}
					if cleanFP != "" && certFingerprint(cert) != cleanFP {
						return fmt.Errorf("mTLS certificate fingerprint mismatch: got %s expected %s", certFingerprint(cert), cleanFP)
					}
					return nil
				},
			},
		},
	}
}

func (cm *ClusterManager) ConnectNode(name, hostname, endpointURL, ipAddr, role, fingerprint, bearerSecret string) (*ManagedClusterNode, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(endpointURL) == "" {
		return nil, errors.New("name and endpointUrl are required")
	}
	if role == "" {
		role = "edge"
	}
	if role != "hub" && role != "edge" && role != "database" {
		return nil, errors.New("role must be hub, edge, or database")
	}
	if strings.TrimSpace(fingerprint) == "" {
		return nil, errors.New("certFingerprint is required for mTLS pinning")
	}

	client := cm.PinnedHTTPClient(fingerprint)
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(endpointURL, "/")+"/api/v1/cluster/node/health", nil)
	if err != nil {
		return nil, err
	}
	if bearerSecret != "" {
		req.Header.Set("Authorization", "Bearer "+bearerSecret)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mTLS connect probe failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned HTTP %d during mTLS probe", resp.StatusCode)
	}
	var health struct {
		Hostname     string         `json:"hostname"`
		PanelVersion string         `json:"panelVersion"`
		Services     []ServiceState `json:"services"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&health)

	cm.mu.Lock()
	defer cm.mu.Unlock()
	id := fmt.Sprintf("node-%d", cm.nextSeq)
	cm.nextSeq++
	if hostname == "" {
		hostname = health.Hostname
	}
	ver := health.PanelVersion
	if ver == "" {
		ver = PanelVersion
	}
	node := &ManagedClusterNode{
		ID:              id,
		Name:            name,
		Hostname:        hostname,
		EndpointURL:     strings.TrimRight(endpointURL, "/"),
		IPAddress:       ipAddr,
		Role:            role,
		Status:          "online",
		PanelVersion:    ver,
		CertFingerprint: strings.ToLower(strings.TrimSpace(fingerprint)),
		Services:        health.Services,
		LastPollAt:      time.Now().UTC(),
	}
	cm.nodes[id] = node
	cp := *node
	return &cp, nil
}

func (cm *ClusterManager) ListNodes() []*ManagedClusterNode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	out := make([]*ManagedClusterNode, 0, len(cm.nodes))
	for _, n := range cm.nodes {
		cp := *n
		out = append(out, &cp)
	}
	return out
}

// PollNodes probes each registered cluster node over pinned TLS 1.3 mTLS and
// transitions unreachable nodes to "offline" so failover decisions use live health.
func (cm *ClusterManager) PollNodes(bearerSecret string) []ManagedClusterNode {
	cm.mu.RLock()
	snapshot := make([]*ManagedClusterNode, 0, len(cm.nodes))
	for _, n := range cm.nodes {
		cp := *n
		snapshot = append(snapshot, &cp)
	}
	cm.mu.RUnlock()

	results := make([]ManagedClusterNode, 0, len(snapshot))
	for _, n := range snapshot {
		client := cm.PinnedHTTPClient(n.CertFingerprint)
		req, err := http.NewRequest(http.MethodGet, n.EndpointURL+"/api/v1/cluster/node/health", nil)
		now := time.Now().UTC()
		nextStatus := "offline"
		var services []ServiceState
		var panelVer string
		if err == nil {
			if bearerSecret != "" {
				req.Header.Set("Authorization", "Bearer "+bearerSecret)
			}
			resp, doErr := client.Do(req)
			if doErr == nil {
				if resp.StatusCode == http.StatusOK {
					var health struct {
						PanelVersion string         `json:"panelVersion"`
						Services     []ServiceState `json:"services"`
					}
					_ = json.NewDecoder(resp.Body).Decode(&health)
					nextStatus = "online"
					services = health.Services
					panelVer = health.PanelVersion
				}
				_ = resp.Body.Close()
			}
		}

		cm.mu.Lock()
		if live, ok := cm.nodes[n.ID]; ok {
			live.Status = nextStatus
			live.LastPollAt = now
			if nextStatus == "online" {
				live.Services = services
				if panelVer != "" {
					live.PanelVersion = panelVer
				}
			}
			results = append(results, *live)
		}
		cm.mu.Unlock()
	}
	return results
}

// SelectHealthyNode returns the preferred node when online, or fails over to
// another healthy online node when available.
func (cm *ClusterManager) SelectHealthyNode(preferredID string) (*ManagedClusterNode, string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if preferredID != "" && preferredID != "auto" {
		if pref, ok := cm.nodes[preferredID]; ok && pref.Status == "online" {
			cp := *pref
			return &cp, "", nil
		}
	}
	for _, n := range cm.nodes {
		if n.ID != preferredID && n.Status == "online" {
			cp := *n
			failoverFrom := ""
			if preferredID != "" && preferredID != "auto" {
				failoverFrom = preferredID
			}
			return &cp, failoverFrom, nil
		}
	}
	return nil, "", errors.New("no healthy online cluster node available")
}

func (state *ServerState) clusterManager() *ClusterManager {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.Cluster == nil {
		cm, err := NewClusterManager()
		if err == nil {
			state.Cluster = cm
		}
	}
	return state.Cluster
}

func archiveDirectoryTarGz(sourceDir string) ([]byte, string, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	cleanBase := filepath.Clean(sourceDir)
	err := filepath.Walk(cleanBase, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(cleanBase, path)
		if err != nil || rel == "." {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, f)
			_ = f.Close()
			return copyErr
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	if err := gz.Close(); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:]), nil
}

func extractTarGzSafely(archive []byte, destDir string) (int, error) {
	gr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return 0, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	cleanDest := filepath.Clean(destDir)
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		target := filepath.Join(cleanDest, filepath.Clean(hdr.Name))
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return count, errors.New("tar entry escapes destination directory")
		}
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return count, err
			}
			continue
		}
		if hdr.Typeflag == tar.TypeReg {
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return count, err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				return count, err
			}
			if _, err := io.Copy(out, io.LimitReader(tr, 50*1024*1024)); err != nil {
				_ = out.Close()
				return count, err
			}
			_ = out.Close()
			count++
		}
	}
	return count, nil
}

func (state *ServerState) handleClusterConnectNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name            string `json:"name"`
		Hostname        string `json:"hostname"`
		EndpointURL     string `json:"endpointUrl"`
		IPAddress       string `json:"ipAddress"`
		Role            string `json:"role"`
		CertFingerprint string `json:"certFingerprint"`
		NodeToken       string `json:"nodeToken"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid connect payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	node, err := state.clusterManager().ConnectNode(req.Name, req.Hostname, req.EndpointURL, req.IPAddress, req.Role, req.CertFingerprint, req.NodeToken)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusBadGateway)
		return
	}
	state.AuditLogger.Append(state.actorUsername(r), "cluster.node.connect", "cluster_node", node.ID,
		fmt.Sprintf(`{"name":%q,"fingerprint":%q}`, node.Name, node.CertFingerprint), clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(node)
}

func (state *ServerState) handleClusterPollNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		NodeToken string `json:"nodeToken"`
	}
	_ = decodeJSONBody(w, r, &req)
	nodes := state.clusterManager().PollNodes(req.NodeToken)
	online := 0
	offline := 0
	for _, n := range nodes {
		if n.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"polled":  len(nodes),
		"online":  online,
		"offline": offline,
		"nodes":   nodes,
	})
}

func (state *ServerState) handleClusterNodeHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	host, _ := os.Hostname()
	services, _ := probeServices()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"hostname":     host,
		"panelVersion": PanelVersion,
		"services":     services,
		"timestamp":    time.Now().UTC(),
	})
}

func (state *ServerState) handleClusterDNSSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		NodeToken string `json:"nodeToken"`
	}
	_ = decodeJSONBody(w, r, &req)

	cm := state.clusterManager()
	nodes := cm.ListNodes()
	zones := state.DNSManager.ListZones()
	syncedNodes := 0
	for _, node := range nodes {
		if node.Status != "online" {
			continue
		}
		payload, _ := json.Marshal(map[string]any{"zones": zones})
		client := cm.PinnedHTTPClient(node.CertFingerprint)
		httpReq, err := http.NewRequest(http.MethodPost, node.EndpointURL+"/api/v1/cluster/node/dns-apply", bytes.NewReader(payload))
		if err != nil {
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if req.NodeToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+req.NodeToken)
		}
		resp, err := client.Do(httpReq)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				syncedNodes++
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"zonesCount":  len(zones),
		"syncedNodes": syncedNodes,
		"totalNodes":  len(nodes),
	})
}

func (state *ServerState) handleClusterDNSApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Zones []dns.Zone `json:"zones"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid DNS apply payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	applied := 0
	for _, z := range req.Zones {
		if z.Name == "" {
			continue
		}
		if _, err := state.DNSManager.CreateZone(z.Name, z.TenantID, z.PrimaryNS, z.AdminEmail, "127.0.0.1"); err == nil {
			applied++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"applied": applied})
}

func (state *ServerState) handleClusterMigrateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username      string `json:"username"`
		TargetNodeID  string `json:"targetNodeId"`
		NodeToken     string `json:"nodeToken"`
		AllowFailover bool   `json:"allowFailover"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid migration payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	acc, err := state.HostingManager.GetAccount(req.Username)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusNotFound)
		return
	}

	cm := state.clusterManager()
	var targetNode *ManagedClusterNode
	var failoverFrom string

	if req.AllowFailover || req.TargetNodeID == "auto" {
		selected, fromID, selErr := cm.SelectHealthyNode(req.TargetNodeID)
		if selErr != nil {
			writeAPIError(w, selErr.Error(), http.StatusBadGateway)
			return
		}
		targetNode = selected
		failoverFrom = fromID
	} else {
		cm.mu.RLock()
		node, ok := cm.nodes[req.TargetNodeID]
		if ok {
			cp := *node
			targetNode = &cp
		}
		cm.mu.RUnlock()
		if !ok {
			writeAPIError(w, "target node not found", http.StatusNotFound)
			return
		}
	}

	bundle, checksum, err := archiveDirectoryTarGz(acc.HomeDir)
	if err != nil {
		writeAPIError(w, "archive account home failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"tenantId":   acc.TenantID,
		"username":   acc.Username,
		"domain":     acc.PrimaryDomain,
		"package":    acc.PackageID,
		"phpVersion": acc.PhpVersion,
		"checksum":   checksum,
		"archiveB64": base64.StdEncoding.EncodeToString(bundle),
	})

	sendMigration := func(node *ManagedClusterNode) (*http.Response, error) {
		client := cm.PinnedHTTPClient(node.CertFingerprint)
		httpReq, reqErr := http.NewRequest(http.MethodPost, node.EndpointURL+"/api/v1/cluster/node/receive-migration", bytes.NewReader(payload))
		if reqErr != nil {
			return nil, reqErr
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if req.NodeToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+req.NodeToken)
		}
		return client.Do(httpReq)
	}

	resp, err := sendMigration(targetNode)
	if err != nil && req.AllowFailover {
		cm.mu.Lock()
		if live, ok := cm.nodes[targetNode.ID]; ok {
			live.Status = "offline"
		}
		cm.mu.Unlock()
		if fallback, fromID, fbErr := cm.SelectHealthyNode(targetNode.ID); fbErr == nil {
			failoverFrom = fromID
			targetNode = fallback
			resp, err = sendMigration(targetNode)
		}
	}
	if err != nil {
		writeAPIError(w, "mTLS migration transfer failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		writeAPIError(w, fmt.Sprintf("target node rejected migration (%d): %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))), http.StatusBadGateway)
		return
	}

	state.AuditLogger.Append(state.actorUsername(r), "cluster.account.migrate", "account", acc.Username,
		fmt.Sprintf(`{"targetNode":%q,"failoverFrom":%q,"checksum":%q}`, targetNode.ID, failoverFrom, checksum), clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"migrated":     true,
		"username":     acc.Username,
		"targetNodeId": targetNode.ID,
		"failoverFrom": failoverFrom,
		"checksum":     checksum,
	})
}

func (state *ServerState) handleClusterReceiveMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TenantID   string `json:"tenantId"`
		Username   string `json:"username"`
		Domain     string `json:"domain"`
		Package    string `json:"package"`
		PhpVersion string `json:"phpVersion"`
		Checksum   string `json:"checksum"`
		ArchiveB64 string `json:"archiveB64"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid migration package: "+err.Error(), http.StatusBadRequest)
		return
	}
	rawArchive, err := base64.StdEncoding.DecodeString(req.ArchiveB64)
	if err != nil {
		writeAPIError(w, "Invalid base64 archive", http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256(rawArchive)
	actualChecksum := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actualChecksum, strings.TrimSpace(req.Checksum)) {
		writeAPIError(w, "Archive SHA-256 checksum mismatch", http.StatusBadRequest)
		return
	}

	acc, err := state.HostingManager.CreateAccount(hosting.CreateAccountRequest{
		TenantID:   req.TenantID,
		Username:   req.Username,
		Domain:     req.Domain,
		Package:    req.Package,
		PhpVersion: req.PhpVersion,
	})
	if err != nil {
		writeAPIError(w, "Target account creation failed: "+err.Error(), http.StatusConflict)
		return
	}
	filesRestored, err := extractTarGzSafely(rawArchive, acc.HomeDir)
	if err != nil {
		_ = state.HostingManager.TerminateAccount(acc.Username)
		writeAPIError(w, "Target extraction failed (rolled back): "+err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"received":      true,
		"username":      acc.Username,
		"filesRestored": filesRestored,
		"checksum":      actualChecksum,
	})
}
