// Copyright (c) 2015-2022 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/minio/mc/pkg/probe"
	"github.com/minio/pkg/v3/env"

	"github.com/mitchellh/go-homedir"
)

// mcCustomConfigDir contains the whole path to config dir. Only access via get/set functions.
var mcCustomConfigDir string

// setMcConfigDir - set a custom MinIO Client config folder.
func setMcConfigDir(configDir string) {
	mcCustomConfigDir = configDir
}

// getMcConfigDir - construct MinIO Client config folder.
func getMcConfigDir() (string, *probe.Error) {
	if mcCustomConfigDir != "" {
		return mcCustomConfigDir, nil
	}
	homeDir, e := homedir.Dir()
	if e != nil {
		return "", probe.NewError(e)
	}
	configDir := filepath.Join(homeDir, defaultMCConfigDir())
	return configDir, nil
}

// Return default default mc config directory.
// Generally you want to use getMcConfigDir which returns custom overrides.
func defaultMCConfigDir() string {
	if runtime.GOOS == "windows" {
		// For windows the path is slightly different
		cmd := filepath.Base(os.Args[0])
		if strings.HasSuffix(strings.ToLower(cmd), ".exe") {
			cmd = cmd[:strings.LastIndex(cmd, ".")]
		}
		return fmt.Sprintf("%s\\", cmd)
	}
	return fmt.Sprintf(".%s/", filepath.Base(os.Args[0]))
}

// mustGetMcConfigDir - construct MinIO Client config folder or fail
func mustGetMcConfigDir() (configDir string) {
	configDir, err := getMcConfigDir()
	fatalIf(err.Trace(), "Unable to get mcConfigDir.")

	return configDir
}

// createMcConfigDir - create MinIO Client config folder
func createMcConfigDir() *probe.Error {
	p, err := getMcConfigDir()
	if err != nil {
		return err.Trace()
	}
	if e := os.MkdirAll(p, 0o700); e != nil {
		return probe.NewError(e)
	}
	return nil
}

// getMcConfigPath - construct MinIO Client configuration path
func getMcConfigPath() (string, *probe.Error) {
	if mcCustomConfigDir != "" {
		return filepath.Join(mcCustomConfigDir, globalMCConfigFile), nil
	}
	dir, err := getMcConfigDir()
	if err != nil {
		return "", err.Trace()
	}
	return filepath.Join(dir, globalMCConfigFile), nil
}

// mustGetMcConfigPath - similar to getMcConfigPath, ignores errors
func mustGetMcConfigPath() string {
	path, err := getMcConfigPath()
	fatalIf(err.Trace(), "Unable to get mcConfigPath.")

	return path
}

// newMcConfig - initializes a new version '10' config.
func newMcConfig() *configV10 {
	cfg := newConfigV10()
	cfg.loadDefaults()
	return cfg
}

// loadMcConfigCached - returns loadMcConfig with a closure for config cache.
func loadMcConfigFactory() func() (*configV10, *probe.Error) {
	// Load once and cache in a closure.
	cfgCache, err := loadConfigV10()

	// loadMcConfig - reads configuration file and returns config.
	return func() (*configV10, *probe.Error) {
		return cfgCache, err
	}
}

// loadMcConfig - returns configuration, initialized later.
var loadMcConfig func() (*configV10, *probe.Error)

// saveMcConfig - saves configuration file and returns error if any.
func saveMcConfig(config *configV10) *probe.Error {
	if config == nil {
		return errInvalidArgument().Trace()
	}

	err := createMcConfigDir()
	if err != nil {
		return err.Trace(mustGetMcConfigDir())
	}

	// Save the config.
	if err := saveConfigV10(config); err != nil {
		return err.Trace(mustGetMcConfigPath())
	}

	// Refresh the config cache.
	loadMcConfig = loadMcConfigFactory()
	return nil
}

// isMcConfigExists returns err if config doesn't exist.
func isMcConfigExists() bool {
	configFile, err := getMcConfigPath()
	if err != nil {
		return false
	}
	if _, e := os.Stat(configFile); e != nil {
		return false
	}
	return true
}

// cleanAlias removes any forbidden trailing slashes or backslashes
// before any validation to avoid annoying mc complaints.
func cleanAlias(s string) string {
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, "\\")
	return s
}

// isValidAlias - Check if alias valid.
func isValidAlias(alias string) bool {
	return regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9-_]*$").MatchString(alias)
}

// getAliasConfig retrieves host specific configuration such as access keys, signature type.
func getAliasConfig(alias string) (*aliasConfigV10, *probe.Error) {
	mcCfg, err := loadMcConfig()
	if err != nil {
		return nil, err.Trace(alias)
	}

	// if host is exact return quickly.
	if _, ok := mcCfg.Aliases[alias]; ok {
		hostCfg := mcCfg.Aliases[alias]
		hostCfg.Src = mustGetMcConfigPath()
		return &hostCfg, nil
	}

	// return error if cannot be matched.
	return nil, errNoMatchingHost(alias).Trace(alias)
}

// mustGetHostConfig retrieves host specific configuration such as access keys, signature type.
func mustGetHostConfig(alias string) *aliasConfigV10 {
	// look for it in the environment variable first.
	aliasCfg, _ := expandAliasFromEnv(env.Get(mcEnvHostPrefix+alias, ""))

	// If alias is not found,
	// look for it in the customized configuration.
	if aliasCfg == nil {
		aliasCfg = aliasToConfigMap[alias]
	}

	if aliasCfg == nil {
		aliasCfg, _ = getAliasConfig(alias)
	}
	return aliasCfg
}

var (
	hostKeys      = regexp.MustCompile("^(https?://)(.*?):(.*)@(.*?)$")
	hostKeyTokens = regexp.MustCompile("^(https?://)(.*?):(.*?):(.*)@(.*?)$")
)

// parse url usually obtained from env.
func parseEnvURLStr(envURL string) (*url.URL, string, string, string, *probe.Error) {
	var accessKey, secretKey, sessionToken string
	var parsedURL string
	if hostKeyTokens.MatchString(envURL) {
		parts := hostKeyTokens.FindStringSubmatch(envURL)
		if len(parts) != 6 {
			return nil, "", "", "", errInvalidArgument().Trace(envURL)
		}
		accessKey = parts[2]
		secretKey = parts[3]
		sessionToken = parts[4]
		parsedURL = fmt.Sprintf("%s%s", parts[1], parts[5])
	} else if hostKeys.MatchString(envURL) {
		parts := hostKeys.FindStringSubmatch(envURL)
		if len(parts) != 5 {
			return nil, "", "", "", errInvalidArgument().Trace(envURL)
		}
		accessKey = parts[2]
		secretKey = parts[3]
		parsedURL = fmt.Sprintf("%s%s", parts[1], parts[4])
	}
	var u *url.URL
	var e error
	if parsedURL != "" {
		u, e = url.Parse(parsedURL)
	} else {
		u, e = url.Parse(envURL)
	}
	if e != nil {
		return nil, "", "", "", probe.NewError(e)
	}
	// Look for if URL has invalid values and return error.
	if !((u.Scheme == "http" || u.Scheme == "https") &&
		(u.Path == "/" || u.Path == "") && u.Opaque == "" &&
		!u.ForceQuery && u.RawQuery == "" && u.Fragment == "") {
		return nil, "", "", "", errInvalidArgument().Trace(u.String())
	}
	if accessKey == "" && secretKey == "" {
		if u.User != nil {
			accessKey = u.User.Username()
			secretKey, _ = u.User.Password()
		}
	}
	return u, accessKey, secretKey, sessionToken, nil
}

const (
	mcEnvHostPrefix = "MC_HOST_"
	mcEnvConfigFile = "MC_CONFIG_ENV_FILE"
)

var aliasToConfigMap = make(map[string]*aliasConfigV10)

func readAliasesFromFile(envConfigFile string) *probe.Error {
	r, e := os.Open(envConfigFile)
	if e != nil {
		return probe.NewError(e).Trace(envConfigFile)
	}
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		envLine := scanner.Text()
		strs := strings.SplitN(envLine, "=", 2)
		if len(strs) != 2 {
			return probe.NewError(fmt.Errorf("parsing error at %s", envLine)).Trace(envConfigFile)
		}
		alias := strings.TrimPrefix(strs[0], mcEnvHostPrefix)
		if len(alias) == 0 {
			return probe.NewError(fmt.Errorf("parsing error at %s", envLine)).Trace(envConfigFile)
		}
		aliasConfig, err := expandAliasFromEnv(strs[1])
		if err != nil {
			return err.Trace(envLine)
		}
		aliasConfig.Src = envConfigFile
		aliasToConfigMap[alias] = aliasConfig
	}
	if e := scanner.Err(); e != nil {
		return probe.NewError(e).Trace(envConfigFile)
	}
	return nil
}

func expandAliasFromEnv(envURL string) (*aliasConfigV10, *probe.Error) {
	u, accessKey, secretKey, sessionToken, err := parseEnvURLStr(envURL)
	if err != nil {
		return nil, err.Trace(envURL)
	}
	if globalCredsEnv {
		if accessKeyVal := os.Getenv(accessKey); accessKeyVal != "" {
			accessKey = accessKeyVal
		}
		if secretKeyVal := os.Getenv(secretKey); secretKeyVal != "" {
			secretKey = secretKeyVal
		}
	}
	return &aliasConfigV10{
		URL:          u.String(),
		API:          "S3v4",
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		SessionToken: sessionToken,
		Src:          "env",
	}, nil
}

// expandAlias expands aliased URL if any match is found, returns as is otherwise.
func expandAlias(aliasedURL string) (alias, urlStr string, aliasCfg *aliasConfigV10, err *probe.Error) {
	// Extract alias from the URL.
	alias, path := url2Alias(aliasedURL)

	if env.IsSet(mcEnvHostPrefix + alias) {
		aliasCfg, err = expandAliasFromEnv(env.Get(mcEnvHostPrefix+alias, ""))
		if err != nil {
			return "", "", nil, err.Trace(aliasedURL)
		}
		return alias, urlJoinPath(aliasCfg.URL, path), aliasCfg, nil
	}

	aliasCfg = aliasToConfigMap[alias]
	if aliasCfg != nil {
		return alias, urlJoinPath(aliasCfg.URL, path), aliasCfg, nil
	}

	// Find the matching alias entry and expand the URL.
	if aliasCfg = mustGetHostConfig(alias); aliasCfg != nil {
		return alias, urlJoinPath(aliasCfg.URL, path), aliasCfg, nil
	}

	return "", aliasedURL, nil, nil // No matching entry found. Return original URL as is.
}

// mustExpandAlias expands aliased URL if any match is found, returns as is otherwise.
func mustExpandAlias(aliasedURL string) (alias, urlStr string, aliasCfg *aliasConfigV10) {
	alias, urlStr, aliasCfg, _ = expandAlias(aliasedURL)
	return alias, urlStr, aliasCfg
}
