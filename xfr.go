package main

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

var (
	queried map[string]bool
	all     []dns.RR
)

func axfr(label, server string) []dns.RR {
	t := new(dns.Transfer)
	m := new(dns.Msg)
	m.SetAxfr(dns.Fqdn(label))
	ch, err := t.In(m, server)
	if err != nil {
		log.Fatalf("Failed to transfer zone: %s", err)
	}

	var rrs []dns.RR
	for env := range ch {
		if env.Error != nil {
			log.Warnf("AXFR section error (%s): %s", label, env.Error)
			continue
		}
		for _, rr := range env.RR {
			rrs = append(rrs, rr)
		}
	}

	return rrs
}

// RecAXFR performs an AXFR on the given label and all of its children and writes the zone file to disk
func RecAXFR(label, server string) []dns.RR {
	log.Infof("Attempting recursive AXFR for %s", label)

	// Reset state
	queried = make(map[string]bool)
	all = make([]dns.RR, 0)

	dir := fmt.Sprintf("%s_%s_recaxfr", strings.TrimPrefix(label, "."), strings.ReplaceAll(time.Now().Format(time.UnixDate), " ", "-"))

	// Create recursive AXFR directory if it doesn't exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.Fatalf("creating recaxfr directory: %s", err)
		}
	}

	addToTree(label, dir, server)
	log.Infof("AXFR complete, %d records saved to %s", len(all), dir)

	return all
}

func addToTree(label, dir, server string) {
	label = dns.Fqdn(label)
	if queried[label] {
		return
	}
	log.Printf("AXFR %s", label)
	queried[label] = true
	rrs := axfr(label, server)

	// Write RRs to zone file
	if len(rrs) > 0 {
		var zoneFile string
		for _, rr := range rrs {
			zoneFile += rr.String() + "\n"
		}
		if err := os.WriteFile(
			path.Join(dir, strings.TrimSuffix(label, ".")+".zone"),
			[]byte(zoneFile),
			0644,
		); err != nil {
			log.Fatalf("Failed to write zone file: %s", err)
		}
	}

	for _, rr := range rrs {
		all = append(all, rr)
		if _, ok := rr.(*dns.NS); ok {
			addToTree(rr.Header().Name, dir, server)
		}
	}
}
