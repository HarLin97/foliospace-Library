package service

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"foliospace-reader/internal/domain"
	"foliospace-reader/internal/launchprofile"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Service) mameContentRegistry() launchprofile.MAMEContentRegistry {
	if s.configDir == "" {
		return launchprofile.DefaultMAMEContentRegistry()
	}
	r, err := launchprofile.ReadMAMEContentRegistry(filepath.Join(s.configDir, "policies", "mame-content-equivalence.json"))
	if errors.Is(err, os.ErrNotExist) {
		return launchprofile.DefaultMAMEContentRegistry()
	}
	if err != nil {
		return launchprofile.MAMEContentRegistry{}
	}
	return r
}

func matchingEquivalentMAMERuntime(profile domain.GameLaunchProfile, req domain.GameLaunchResolveRequest, registry launchprofile.MAMEContentRegistry) (domain.GameRuntimeDescriptor, *domain.GameLaunchContentAudit, bool) {
	approved := profile.Runtime
	if approved.ID != "mame" || approved.ContentSet != "mame-"+approved.Version || profile.Policy != launchprofile.MAMEPolicyForVersion(approved.Version) {
		return domain.GameRuntimeDescriptor{}, nil, false
	}
	for _, runtime := range req.Runtimes {
		if runtime.ID != "mame" || runtime.Version == approved.Version || runtime.ContentSet != "mame-"+runtime.Version {
			continue
		}
		// Reuse all existing client/platform/minimum-version and core identity checks.
		probe := req
		probe.Runtimes = []domain.GameRuntimeDescriptor{runtime}
		probe.Runtimes[0].Version = approved.Version
		probe.Runtimes[0].ContentSet = approved.ContentSet
		if _, ok := matchingPersistedRuntime(profile, probe); !ok {
			continue
		}
		source, target, ok := registry.Equivalent(profile.CanonicalSet, approved.Version, runtime.Version)
		if !ok {
			continue
		}
		rev, err := strconv.ParseUint(source.ListXMLSHA256[:8], 16, 32)
		expectedRevision := int(rev & 0x7fffffff)
		if expectedRevision == 0 {
			expectedRevision = 1
		}
		if err != nil || profile.Revision != expectedRevision || !strings.HasSuffix(profile.ID, "-"+source.ListXMLSHA256[:8]) {
			continue
		}
		return runtime, &domain.GameLaunchContentAudit{Method: "mame-content-equivalence-v1", Policy: profile.Policy, SourceRuntime: approved, SourceListXMLSHA256: source.ListXMLSHA256, RequestedListXMLSHA256: target.ListXMLSHA256, DefinitionSHA256: source.DefinitionSHA256}, true
	}
	return domain.GameRuntimeDescriptor{}, nil, false
}

// Cross-version reuse verifies actual bytes rather than trusting catalog hashes.
func equivalentMAMESourceMatches(path string, size int64, expected string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() != size {
		return false
	}
	h := sha1.New()
	n, err := io.Copy(h, io.LimitReader(f, size+1))
	if err != nil || n != size || hex.EncodeToString(h.Sum(nil)) != strings.ToLower(expected) {
		return false
	}
	after, err := os.Stat(path)
	return err == nil && os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}
