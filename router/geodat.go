package router

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GeoCategory is a matching geosite category found in a dat file.
type GeoCategory struct {
	DatFile string // e.g., "geosite_v2fly.dat"
	Tag     string // lowercase, e.g., "youtube"
}

// Entry returns the xray routing entry string, e.g. "ext:geosite_v2fly.dat:youtube".
func (gc GeoCategory) Entry() string {
	return "ext:" + gc.DatFile + ":" + gc.Tag
}

// Label returns a short human-readable label for inline buttons.
func (gc GeoCategory) Label() string {
	name := strings.TrimPrefix(gc.DatFile, "geosite_")
	name = strings.TrimSuffix(name, ".dat")
	return gc.Tag + " (" + name + ")"
}

// LookupDomain searches domain in all geosite .dat files in datDir using xk-geodat.
func LookupDomain(geodatTool, datDir, domain string) ([]GeoCategory, error) {
	entries, err := os.ReadDir(datDir)
	if err != nil {
		return nil, fmt.Errorf("read dat dir: %w", err)
	}

	var results []GeoCategory
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".dat") {
			continue
		}
		// Skip IP databases
		name := e.Name()
		if strings.HasPrefix(name, "geoip_") || strings.HasPrefix(name, "zkeenip") {
			continue
		}
		cats, err := lookupInFile(geodatTool, filepath.Join(datDir, name), name, domain)
		if err != nil {
			continue // not a geosite file or other error — skip
		}
		results = append(results, cats...)
	}
	return results, nil
}

type geodatResponse struct {
	OK      bool `json:"ok"`
	Matches []struct {
		Tag string `json:"tag"`
	} `json:"matches"`
}

func lookupInFile(geodatTool, path, filename, domain string) ([]GeoCategory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	out, err := run(ctx, geodatTool, "lookup", "--kind", "geosite", "--path", path, "--value", domain)
	if err != nil {
		return nil, err
	}

	var resp geodatResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, nil
	}

	cats := make([]GeoCategory, 0, len(resp.Matches))
	for _, m := range resp.Matches {
		cats = append(cats, GeoCategory{
			DatFile: filename,
			Tag:     strings.ToLower(m.Tag),
		})
	}
	return cats, nil
}
