package main
import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)
type Config struct {
	Token        string `json:"token"`
	DiscordToken string `json:"discordToken"`
	Password     string `json:"password"`
	GuildID      string `json:"guildId"`
	DiscordHost  string `json:"discordHost"`
	APIVersion   string `json:"apiVersion"`
	OS           string `json:"os"`
	Browser      string `json:"browser"`
	Device       string `json:"device"`
	SalvoSize    int    `json:"salvoSize"`
}
func (c *Config) GetToken() string {
	if c.Token != "" {
		return c.Token
	}
	return c.DiscordToken
}
func (c *Config) GetHost() string {
	if c.DiscordHost != "" {
		return c.DiscordHost
	}
	return "canary.discord.com"
}
func (c *Config) GetAPIVersion() string {
	if c.APIVersion != "" {
		return c.APIVersion
	}
	return "v9"
}
func (c *Config) UserAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0"
}
func (c *Config) BuildSuperProperties() string {
	if cachedSuperProperties != "" {
		return cachedSuperProperties
	}
	releaseChannel := "stable"
	if strings.HasPrefix(c.GetHost(), "canary") {
		releaseChannel = "canary"
	}
	osName := c.OS
	if osName == "" {
		osName = "Windows"
	}
	browserName := c.Browser
	if browserName == "" {
		browserName = "Firefox"
	}
	props := map[string]interface{}{
		"os":                       osName,
		"browser":                  browserName,
		"device":                   c.Device,
		"system_locale":            "en-US",
		"browser_user_agent":       c.UserAgent(),
		"browser_version":          "133.0",
		"os_version":               "10",
		"referrer":                 "https://www.google.com/",
		"referring_domain":         "www.google.com",
		"search_engine":            "google",
		"referrer_current":         "",
		"referring_domain_current": "",
		"release_channel":          releaseChannel,
		"client_build_number":      356140,
		"client_event_source":      nil,
		"has_client_mods":          false,
	}
	data, _ := json.Marshal(props)
	cachedSuperProperties = base64.StdEncoding.EncodeToString(data)
	return cachedSuperProperties
}
func loadConfigFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("[MFA ERROR] Failed to read %s: %v\n", path, err)
		return false
	}
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("[MFA ERROR] Failed to parse %s: %v\n", path, err)
		return false
	}
	cachedSuperProperties = ""
	loadCachedMfaToken()
	rebuildHotCache()
	return true
}
func loadCachedMfaToken() {
	if data, err := os.ReadFile("mfa.txt"); err == nil && len(data) > 0 {
		cachedMfaToken = strings.TrimSpace(string(data))
	}
}
