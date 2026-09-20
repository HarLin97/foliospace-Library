// Generates compatibility evidence from locally installed, trusted official
// listxml releases. It does not scan ROMs, modify profiles, or access the network.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"foliospace-reader/internal/launchprofile"
	"log"
	"os"
	"sort"
	"strings"
)

func main() {
	xmls := flag.String("listxml", "", "comma-separated trusted listxml XML/ZIP paths")
	sets := flag.String("sets", "", "comma-separated audited game short names")
	existing := flag.String("existing", "", "optional existing registry to retain other definitions")
	flag.Parse()
	if *xmls == "" || *sets == "" {
		log.Fatal("--listxml and --sets are required")
	}
	registry := launchprofile.MAMEContentRegistry{SchemaVersion: 1}
	if *existing != "" {
		var err error
		registry, err = launchprofile.ReadMAMEContentRegistry(*existing)
		if err != nil {
			log.Fatal(err)
		}
	}
	requested := strings.Split(*sets, ",")
	for i := range requested {
		requested[i] = strings.TrimSpace(requested[i])
	}
	for _, path := range strings.Split(*xmls, ",") {
		c, err := launchprofile.ParseMAMEListXMLFile(strings.TrimSpace(path), requested)
		if err != nil {
			log.Fatal(err)
		}
		for _, set := range requested {
			d, err := c.ContentDefinition(set)
			if err != nil {
				log.Fatal(err)
			}
			found := false
			for _, old := range registry.Definitions {
				if old.Set == d.Set && old.Version == d.Version {
					if old != d {
						log.Fatal("conflicting definition for ", d.Set, " ", d.Version)
					}
					found = true
				}
			}
			if !found {
				registry.Definitions = append(registry.Definitions, d)
			}
		}
	}
	if err := registry.Validate(); err != nil {
		log.Fatal(err)
	}
	sort.Slice(registry.Definitions, func(i, j int) bool {
		a, b := registry.Definitions[i], registry.Definitions[j]
		if a.Set == b.Set {
			return a.Version < b.Version
		}
		return a.Set < b.Set
	})
	b, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(os.Stdout, string(b))
}
