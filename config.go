package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Token           string `json:"token"`
	DiscordToken    string `json:"discordToken"`
	Password        string `json:"password"`
	GuildID         string `json:"guildId"`
	DiscordHost     string `json:"discordHost"`
	APIVersion      string `json:"apiVersion"`
	OS              string `json:"os"`
	Browser         string `json:"browser"`
	Device          string `json:"device"`
	SalvoSize       int    `json:"salvoSize"`
	SnipeCooldownMs int    `json:"snipeCooldownMs"`
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
	if cachedProps != "" {
		return cachedProps
	}
	rel := "stable"
	if strings.HasPrefix(c.GetHost(), "canary") {
		rel = "canary"
	}
	sys := c.OS
	if sys == "" {
		sys = "Windows"
	}
	br := c.Browser
	if br == "" {
		br = "Firefox"
	}
	m := map[string]interface{}{
		"os":                       sys,
		"browser":                  br,
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
		"release_channel":          rel,
		"client_build_number":      356140,
		"client_event_source":      nil,
		"has_client_mods":          false,
	}
	b, _ := json.Marshal(m)
	cachedProps = base64.StdEncoding.EncodeToString(b)
	return cachedProps
}

func loadConfigFile(fn string) bool {
	b, err := os.ReadFile(fn)
	if err != nil {
		fmt.Printf("[MFA ERROR] Failed to read %s: %v\n", fn, err)
		return false
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		fmt.Printf("[MFA ERROR] Failed to parse %s: %v\n", fn, err)
		return false
	}
	cachedProps = ""
	loadCachedMfaToken()
	rebuildHotCache()
	return true
}

func loadCachedMfaToken() {
	if b, err := os.ReadFile("mfa.txt"); err == nil && len(b) > 0 {
		cachedMFA = strings.TrimSpace(string(b))
	}
}
