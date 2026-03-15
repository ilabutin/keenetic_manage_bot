package router

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	Name    string
	IP      string
	MAC     string
	Network string // human-readable network name from interface block
	RxBytes uint64
	TxBytes uint64
}

// WireguardPeer represents a configured WireGuard peer.
type WireguardPeer struct {
	Description    string
	Online         bool
	RemoteEndpoint string
	RxBytes        uint64
	TxBytes        uint64
}

// WireguardPeers returns all configured peers from `ndmc -c "show interface <iface>"`.
func WireguardPeers(iface string) ([]WireguardPeer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := run(ctx, "/bin/ndmc", "-c", "show interface "+iface)
	if err != nil {
		return nil, fmt.Errorf("ndmc: %w\n%s", err, out)
	}
	return parseWireguardInterface(out), nil
}

// parseWireguardInterface parses `ndmc -c "show interface <iface>"` output.
func parseWireguardInterface(raw string) []WireguardPeer {
	var peers []WireguardPeer
	cur := map[string]string{}
	inPeer := false

	flush := func() {
		defer func() {
			cur = map[string]string{}
			inPeer = false
		}()
		if !inPeer || cur["description"] == "" {
			return
		}
		rx, _ := strconv.ParseUint(cur["rxbytes"], 10, 64)
		tx, _ := strconv.ParseUint(cur["txbytes"], 10, 64)
		peers = append(peers, WireguardPeer{
			Description:    cur["description"],
			Online:         cur["online"] == "yes",
			RemoteEndpoint: cur["remote-endpoint-address"],
			RxBytes:        rx,
			TxBytes:        tx,
		})
	}

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "peer:" {
			flush()
			inPeer = true
			continue
		}
		if line == "summary:" {
			flush()
			break
		}
		if !inPeer {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if val == "" {
			continue
		}
		switch key {
		case "description", "online", "rxbytes", "txbytes", "remote-endpoint-address":
			if _, ok := cur[key]; !ok {
				cur[key] = val
			}
		}
	}
	flush()
	return peers
}

// ConnectedClients returns active clients from `ndmc -c "show ip hotspot"`.
func ConnectedClients() ([]Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := run(ctx, "/bin/ndmc", "-c", "show ip hotspot")
	if err != nil {
		return nil, fmt.Errorf("ndmc: %w\n%s", err, out)
	}
	return parseHotspot(out), nil
}

// parseHotspot parses the indented text output of `ndmc -c "show ip hotspot"`.
//
// Structure of each device block:
//
//	host:
//	  mac: ...   ip: ...   hostname: ...   name: ...   mws-backhaul: ...
//	  interface:
//	    id: Bridge0   name: Home   description: ...
//	    dhcp: ...
//	  registered: ...   active: yes/no   rxbytes: ...   ...
func parseHotspot(raw string) []Client {
	var clients []Client
	cur := map[string]string{}
	inInterface := false

	flush := func() {
		defer func() {
			cur = map[string]string{}
			inInterface = false
		}()
		if cur["active"] == "no" {
			return
		}
		ip := cur["ip"]
		if ip == "" || ip == "0.0.0.0" {
			return
		}
		name := cur["name"]
		if name == "" {
			name = cur["hostname"]
		}
		rx, _ := strconv.ParseUint(cur["rxbytes"], 10, 64)
		tx, _ := strconv.ParseUint(cur["txbytes"], 10, 64)
		clients = append(clients, Client{
			Name:    name,
			IP:      ip,
			MAC:     cur["mac"],
			Network: cur["net_name"],
			RxBytes: rx,
			TxBytes: tx,
		})
	}

	for _, raw := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(raw)

		if line == "host:" {
			flush()
			continue
		}

		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		switch key {
		case "interface":
			inInterface = true
		case "active":
			inInterface = false
			cur["active"] = val
		default:
			if inInterface {
				if key == "name" && val != "" {
					if _, ok := cur["net_name"]; !ok {
						cur["net_name"] = val
					}
				}
			} else {
				switch key {
				case "mac", "ip", "hostname", "name", "rxbytes", "txbytes":
					if _, ok := cur[key]; !ok {
						cur[key] = val
					}
				}
			}
		}
	}
	flush()
	return clients
}
