package naomi2catalog

import "strings"

// Entry describes the canonical package shape for one pinned NAOMI 2 driver.
// Scanner and launch-manifest code must share this source so catalog identity
// cannot drift from dependency validation.
type Entry struct {
	SetName     string
	Title       string
	Region      string
	GDROM       string
	ExpectedPIC string
	Parent      string
}

var entries = map[string]Entry{
	"beachspi":  {Title: "Beach Spikers", Region: "World", GDROM: "gds-0014", ExpectedPIC: "317-0317-com.pic"},
	"clubk2k3":  {Title: "Club Kart: European Session (2003)", Region: "World"},
	"clubk2kp":  {Title: "Club Kart: European Session (2003, Prototype)", Region: "World"},
	"clubk2kpa": {Title: "Club Kart: European Session (2003, Prototype Rev A)", Region: "World"},
	"clubkcyc":  {Title: "Club Kart for Cycraft (Rev A)", Region: "World", GDROM: "gds-0029a", ExpectedPIC: "317-0358-com.pic"},
	"clubkcyco": {Title: "Club Kart for Cycraft", Region: "World", GDROM: "gds-0029", ExpectedPIC: "317-0358-com.pic"},
	"clubkprz":  {Title: "Club Kart Prize", Region: "World"},
	"clubkpzb":  {Title: "Club Kart Prize Version B", Region: "World"},
	"clubkrt":   {Title: "Club Kart: European Session (Rev D)", Region: "World"},
	"clubkrta":  {Title: "Club Kart: European Session (Rev A)", Region: "World", Parent: "clubkrt"},
	"clubkrtc":  {Title: "Club Kart: European Session (Rev C)", Region: "World", Parent: "clubkrt"},
	"clubkrto":  {Title: "Club Kart: European Session", Region: "World", Parent: "clubkrt"},
	"inidv3ca":  {Title: "Initial D Arcade Stage Ver. 3 Cycraft Edition (Rev A)", Region: "World", GDROM: "gds-0039a", ExpectedPIC: "317-0406-com.pic"},
	"inidv3cy":  {Title: "Initial D Arcade Stage Ver. 3 Cycraft Edition (Rev B)", Region: "World", GDROM: "gds-0039b", ExpectedPIC: "317-0406-com.pic"},
	"initdv3e":  {Title: "Initial D Arcade Stage Ver. 3 (Export)", Region: "World", GDROM: "gds-0033", ExpectedPIC: "317-0384-com.pic"},
	"initdv3j":  {Title: "Initial D Arcade Stage Ver. 3 (Japan Rev C)", Region: "Japan", GDROM: "gds-0032c", ExpectedPIC: "317-0379-jpn.pic"},
	"initdv3jb": {Title: "Initial D Arcade Stage Ver. 3 (Japan Rev B)", Region: "Japan", GDROM: "gds-0032b", ExpectedPIC: "317-0379-jpn.pic"},
	"initd":     {Title: "Initial D Arcade Stage (Rev B)", Region: "Japan", GDROM: "gds-0020b", ExpectedPIC: "317-0331-jpn.pic"},
	"initdexp":  {Title: "Initial D Arcade Stage (Export Rev A)", Region: "World", GDROM: "gds-0025a", ExpectedPIC: "317-0343-com.pic"},
	"initdexpo": {Title: "Initial D Arcade Stage (Export)", Region: "World", GDROM: "gds-0025", ExpectedPIC: "317-0343-com.pic"},
	"initdo":    {Title: "Initial D Arcade Stage", Region: "Japan", GDROM: "gds-0020", ExpectedPIC: "317-0331-jpn.pic"},
	"initdv2e":  {Title: "Initial D Arcade Stage Ver. 2 (Export)", Region: "World", GDROM: "gds-0027", ExpectedPIC: "317-0357-exp.pic"},
	"initdv2j":  {Title: "Initial D Arcade Stage Ver. 2 (Rev B)", Region: "Japan", GDROM: "gds-0026b", ExpectedPIC: "317-0345-jpn.pic"},
	"initdv2ja": {Title: "Initial D Arcade Stage Ver. 2 (Rev A)", Region: "Japan", GDROM: "gds-0026a", ExpectedPIC: "317-0345-jpn.pic"},
	"initdv2jo": {Title: "Initial D Arcade Stage Ver. 2", Region: "Japan", GDROM: "gds-0026", ExpectedPIC: "317-0345-jpn.pic"},
	"kingrt66":  {Title: "The King of Route 66 (Rev A)", Region: "World"},
	"kingrt66p": {Title: "The King of Route 66 (Prototype)", Region: "World", Parent: "kingrt66"},
	"soulsurf":  {Title: "Soul Surfer (Rev A)", Region: "World"},
	"vf4":       {Title: "Virtua Fighter 4 (Ver. C)", Region: "World", GDROM: "gds-0012c", ExpectedPIC: "317-0314-com.pic"},
	"vf4b":      {Title: "Virtua Fighter 4 (Rev B)", Region: "World", GDROM: "gds-0012b", ExpectedPIC: "317-0314-com.pic"},
	"vf4cart":   {Title: "Virtua Fighter 4 (Cartridge)", Region: "World"},
	"vf4evo":    {Title: "Virtua Fighter 4: Evolution (Ver. B)", Region: "Japan", GDROM: "gds-0024c", ExpectedPIC: "317-0338-jpn.pic"},
	"vf4evoa":   {Title: "Virtua Fighter 4: Evolution (Rev A)", Region: "Japan", GDROM: "gds-0024a", ExpectedPIC: "317-0338-jpn.pic"},
	"vf4evob":   {Title: "Virtua Fighter 4: Evolution (Ver. B)", Region: "Japan", GDROM: "gds-0024b", ExpectedPIC: "317-0338-jpn.pic"},
	"vf4evoct":  {Title: "Virtua Fighter 4: Evolution (Cartridge)", Region: "World"},
	"vf4o":      {Title: "Virtua Fighter 4", Region: "World", GDROM: "gds-0012", ExpectedPIC: "317-0314-com.pic"},
	"vf4tuned":  {Title: "Virtua Fighter 4: Final Tuned (Ver. B)", Region: "World", GDROM: "gds-0036f", ExpectedPIC: "317-0387-com.pic"},
	"vf4tuneda": {Title: "Virtua Fighter 4: Final Tuned (Rev A)", Region: "World", GDROM: "gds-0036a", ExpectedPIC: "317-0387-com.pic"},
	"vf4tunedd": {Title: "Virtua Fighter 4: Final Tuned (Ver. A)", Region: "World", GDROM: "gds-0036d", ExpectedPIC: "317-0387-com.pic"},
	"vstrik3":   {Title: "Virtua Striker 3", Region: "World", GDROM: "gds-0006", ExpectedPIC: "317-0304-com.bin"},
	"vstrik3c":  {Title: "Virtua Striker 3 (Rev B)", Region: "World"},
	"vstrik3co": {Title: "Virtua Striker 3", Region: "World", Parent: "vstrik3c"},
	"wldrider":  {Title: "Wild Riders", Region: "World"},
}

func init() {
	for setName, entry := range entries {
		entry.SetName = setName
		entries[setName] = entry
	}
}

func Lookup(setName string) (Entry, bool) {
	entry, ok := entries[strings.ToLower(strings.TrimSpace(setName))]
	return entry, ok
}

func Parent(setName string) string {
	entry, ok := Lookup(setName)
	if !ok {
		return ""
	}
	return entry.Parent
}

func Entries() []Entry {
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	return result
}
