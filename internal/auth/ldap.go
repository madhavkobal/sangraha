package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// LDAPConfig holds the configuration for LDAP authentication.
type LDAPConfig struct {
	// Enabled controls whether LDAP authentication is active.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// URL is the LDAP server URL, e.g. ldap://host:389 or ldaps://host:636.
	URL string `yaml:"url" mapstructure:"url"`
	// BindDN is the distinguished name used to bind for searches.
	BindDN string `yaml:"bind_dn" mapstructure:"bind_dn"`
	// BindPassword must be set via the SANGRAHA_AUTH_LDAP_BIND_PASSWORD env var only.
	BindPassword string `yaml:"-" mapstructure:"-"`
	// UserBase is the search base for user entries (e.g. ou=users,dc=example,dc=com).
	UserBase string `yaml:"user_base" mapstructure:"user_base"`
	// UserFilter is the LDAP filter used to find a user by username.
	// Use %s as a placeholder for the username (e.g. (&(objectClass=person)(uid=%s))).
	UserFilter string `yaml:"user_filter" mapstructure:"user_filter"`
	// GroupBase is the search base for group entries.
	GroupBase string `yaml:"group_base" mapstructure:"group_base"`
	// GroupFilter is the LDAP filter used to find groups for a user DN.
	// Use %s as a placeholder for the user DN (e.g. (&(objectClass=groupOfNames)(member=%s))).
	GroupFilter string `yaml:"group_filter" mapstructure:"group_filter"`
	// AdminGroups lists LDAP groups whose members receive admin-level access.
	AdminGroups []string `yaml:"admin_groups" mapstructure:"admin_groups"`
	// TLSInsecure disables TLS certificate verification for ldaps:// connections.
	// Never set this to true in production.
	TLSInsecure bool `yaml:"tls_insecure" mapstructure:"tls_insecure"`
}

// LDAPProvider authenticates users against an LDAP directory.
type LDAPProvider struct {
	cfg LDAPConfig
}

// NewLDAPProvider creates a LDAPProvider with the given configuration.
func NewLDAPProvider(cfg LDAPConfig) *LDAPProvider {
	return &LDAPProvider{cfg: cfg}
}

// Authenticate binds to LDAP as the service account, searches for the user,
// attempts a bind with the user's credentials, then fetches group memberships.
// Returns the list of group names on success, or an error on failure.
func (p *LDAPProvider) Authenticate(ctx context.Context, username, password string) (groups []string, err error) {
	if username == "" || password == "" {
		return nil, errors.New("ldap: username and password must not be empty")
	}

	conn, err := p.dial()
	if err != nil {
		return nil, fmt.Errorf("ldap: dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Respect context cancellation during blocking LDAP calls.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	defer close(done)

	// Bind as service account to search for the user.
	if p.cfg.BindDN != "" {
		if err = conn.Bind(p.cfg.BindDN, p.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap: service bind: %w", err)
		}
	}

	// Search for the user DN.
	userDN, err := p.findUserDN(conn, username)
	if err != nil {
		return nil, err
	}

	// Bind as the user to verify credentials.
	if err = conn.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("ldap: user bind failed: %w", err)
	}

	// Re-bind as service account to search for groups (some servers require this).
	if p.cfg.BindDN != "" {
		if err = conn.Bind(p.cfg.BindDN, p.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap: re-bind for group search: %w", err)
		}
	}

	groups, err = p.findGroups(conn, userDN)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// IsAdmin returns true if any group in groups is listed in cfg.AdminGroups.
func (p *LDAPProvider) IsAdmin(groups []string) bool {
	for _, g := range groups {
		for _, ag := range p.cfg.AdminGroups {
			if strings.EqualFold(g, ag) {
				return true
			}
		}
	}
	return false
}

// dial opens an LDAP connection to cfg.URL.
func (p *LDAPProvider) dial() (*ldap.Conn, error) {
	u := p.cfg.URL
	switch {
	case strings.HasPrefix(u, "ldaps://"):
		host := strings.TrimPrefix(u, "ldaps://")
		tlsCfg := &tls.Config{
			InsecureSkipVerify: p.cfg.TLSInsecure, //nolint:gosec // operator-controlled setting; documented as unsafe
			MinVersion:         tls.VersionTLS12,
		}
		return ldap.DialTLS("tcp", host, tlsCfg)
	case strings.HasPrefix(u, "ldap://"):
		host := strings.TrimPrefix(u, "ldap://")
		return ldap.Dial("tcp", host)
	default:
		return nil, fmt.Errorf("ldap: unsupported URL scheme in %q (use ldap:// or ldaps://)", u)
	}
}

// findUserDN searches for the user and returns their DN.
func (p *LDAPProvider) findUserDN(conn *ldap.Conn, username string) (string, error) {
	filter := fmt.Sprintf(p.cfg.UserFilter, ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		p.cfg.UserBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, // size limit
		0, // time limit
		false,
		filter,
		[]string{"dn"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		return "", fmt.Errorf("ldap: user search: %w", err)
	}
	if len(result.Entries) == 0 {
		return "", fmt.Errorf("ldap: user %q not found", username)
	}
	return result.Entries[0].DN, nil
}

// findGroups returns the cn of each group the user (identified by userDN) belongs to.
func (p *LDAPProvider) findGroups(conn *ldap.Conn, userDN string) ([]string, error) {
	if p.cfg.GroupBase == "" || p.cfg.GroupFilter == "" {
		return nil, nil
	}
	filter := fmt.Sprintf(p.cfg.GroupFilter, ldap.EscapeFilter(userDN))
	req := ldap.NewSearchRequest(
		p.cfg.GroupBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		[]string{"cn"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("ldap: group search: %w", err)
	}
	groups := make([]string, 0, len(result.Entries))
	for _, e := range result.Entries {
		if cn := e.GetAttributeValue("cn"); cn != "" {
			groups = append(groups, cn)
		}
	}
	return groups, nil
}
