// Copyright 2025 Upbound Inc.
// All rights reserved

// Package login contains commands for managing login sessions.
package login

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/golang-jwt/jwt"
	"github.com/mdp/qrterminal/v3"
	"github.com/pkg/browser"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/service/organizations"
	uphttp "github.com/upbound/up/internal/http"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

const (
	defaultTimeout = 30 * time.Second
	loginPath      = "/v1/login"

	webLogin            = "/login"
	issueEndpoint       = "/v1/issueTOTP"
	exchangeEndpoint    = "/v1/checkTOTP"
	totpDisplay         = "/cli/loginCode"
	loginResultEndpoint = "/cli/loginResult"

	errLoginFailed    = "unable to login"
	errReadBody       = "unable to read response body"
	errParseCookieFmt = "unable to parse session cookie: %s"
	errNoIDInToken    = "token is missing ID"
	errUpdateConfig   = "unable to update config file"
	errNoSubInToken   = "no sub claim in token"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *LoginCmd) BeforeApply() error {
	c.stdin = os.Stdin
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply parses flags and sets defaults.
func (c *LoginCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	// Avoid changing the organization for a profile. Encourage the user to
	// create a new profile if they have a second organization.
	profileOrg := upCtx.Profile.Organization
	if profileOrg == "" {
		profileOrg = upCtx.Profile.Account //nolint:staticcheck // Fallback to deprecated field.
	}
	if profileOrg != "" && upCtx.Organization != "" && upCtx.Organization != profileOrg {
		return errors.Errorf("requested organization %q does not match profile organization %q; create a new profile by passing --profile or update the organization with `up profile set`",
			upCtx.Organization, profileOrg)
	}

	// NOTE(hasheddan): client timeout is handled with request context.
	// TODO(hasheddan): we can't use the typical up-sdk-go client here because
	// we need to read session cookie from body. We should add support in the
	// SDK so that we can be consistent across all commands.
	var tr http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: upCtx.InsecureSkipTLSVerify, //nolint:gosec // Let the user be insecure.
		},
	}
	c.client = &http.Client{
		Transport: tr,
	}
	kongCtx.Bind(upCtx)
	if c.Token != "" {
		return nil
	}
	// Only prompt for password if username flag is explicitly passed
	if c.Password == "" && c.Username != "" {
		password, err := c.prompter.Prompt("Password", true)
		if err != nil {
			return err
		}
		c.Password = password
		return nil
	}

	return nil
}

// LoginCmd adds a user or token profile with session token to the up config
// file if a username is passed, but defaults to launching a web browser to authenticate with Upbound.
type LoginCmd struct { //nolint:revive // Can't just call this `Cmd` because `LogoutCmd` is also in this package.
	client   uphttp.Client
	stdin    io.Reader
	prompter input.Prompter

	Username string `env:"UP_USER"     help:"Username used to execute command."                                                          short:"u" xor:"identifier"`
	Password string `env:"UP_PASSWORD" help:"Password for specified user. '-' to read from stdin."                                       short:"p"`
	Token    string `env:"UP_TOKEN"    help:"Upbound API token (personal access token) used to execute command. '-' to read from stdin." short:"t" xor:"identifier"`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`

	UseDeviceCode bool `help:"Use authentication flow based on device code. We will also use this if it can't launch a browser in your behalf, e.g. in remote SSH"`
}

// Run executes the login command.
func (c *LoginCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
	// simple auth using explicit flags
	if c.Username != "" || c.Token != "" {
		return c.simpleAuth(ctx, upCtx)
	}

	if upCtx.Profile.TokenType == profile.TokenTypeRobot {
		p.Printfln("%s logged in to organization %s", upCtx.Profile.ID, upCtx.Organization)
		return nil
	}

	// start webserver listening on port
	token := make(chan string, 1)
	redirect := make(chan string, 1)
	defer close(token)
	defer close(redirect)

	cb := callbackServer{
		token:    token,
		redirect: redirect,
	}
	err := cb.startServer()
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	defer cb.shutdownServer(ctx) //nolint:errcheck // Exiting right after defer anyway.

	resultEP := *upCtx.AccountsEndpoint
	resultEP.Path = loginResultEndpoint
	browser.Stderr = nil
	browser.Stdout = nil
	if c.UseDeviceCode {
		if err = c.handleDeviceLogin(upCtx, token, p); err != nil {
			return err
		}
	} else {
		if err := browser.OpenURL(getEndpoint(*upCtx.AccountsEndpoint, *upCtx.APIEndpoint, fmt.Sprintf("http://localhost:%d", cb.port))); err != nil {
			p.Println("Could not open a browser!")
			if err = c.handleDeviceLogin(upCtx, token, p); err != nil {
				return err
			}
		}
	}

	// wait for response on webserver or timeout
	timeout := uint(5)
	var t string
	select {
	case <-time.After(time.Duration(timeout) * time.Minute):
		break
	case t = <-token:
		break
	}

	if err := c.exchangeTokenForSession(ctx, upCtx, t); err != nil {
		resultEP.RawQuery = url.Values{
			"message": []string{err.Error()},
		}.Encode()
		redirect <- resultEP.String()
		return errors.Wrap(err, errLoginFailed)
	}

	if err := c.validateOrganization(ctx, upCtx); err != nil {
		resultEP.RawQuery = url.Values{
			"message": []string{err.Error()},
		}.Encode()
		redirect <- resultEP.String()
		return errors.Wrap(err, errLoginFailed)
	}
	redirect <- resultEP.String()

	p.Printfln("%s logged in to organization %s", upCtx.Profile.ID, upCtx.Organization)

	return nil
}

func (c *LoginCmd) validateOrganization(ctx context.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}

	orgClient := organizations.NewClient(cfg)
	// upCtx.Account is set during login, so should always contain an
	// organization name.
	_, err = orgClient.GetOrgID(ctx, upCtx.Organization)
	return err
}

// auth is the request body sent to authenticate a user or token.
type auth struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func setSession(ctx context.Context, upCtx *upbound.Context, res *http.Response, tokenType profile.TokenType, authID, token string) error {
	session := token
	var err error

	if res != nil {
		session, err = extractSession(res, upbound.CookieName)
		if err != nil {
			return err
		}
	}

	// If profile name was not provided and no default exists, set name to 'default'.
	if upCtx.ProfileName == "" {
		upCtx.ProfileName = profile.DefaultName
	}

	// Don't overwrite the profile type if there is one. Default to cloud if
	// this is a new profile or it wasn't set.
	profileType := upCtx.Profile.Type
	if profileType == "" {
		profileType = profile.TypeCloud
	}

	// Re-initialize profile for this login.
	profile := profile.Profile{
		ID:        authID,
		Type:      profileType,
		TokenType: tokenType,
		// Set session early so that it can be used to fetch user info if
		// necessary.
		Session: session,
		Domain:  upCtx.Domain.String(),
		// Carry over existing config.
		BaseConfig:      upCtx.Profile.BaseConfig,
		SpaceKubeconfig: upCtx.Profile.SpaceKubeconfig,
	}
	upCtx.Profile = profile

	// If the account (organization) is not set (by profile or flags), try to
	// infer it.
	if upCtx.Organization == "" {
		upCtx.Organization, err = inferOrganization(ctx, upCtx)
		if err != nil {
			return err
		}
	}
	upCtx.Profile.Organization = upCtx.Organization

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, upCtx.Profile); err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	if err := upCtx.Cfg.SetDefaultUpboundProfile(upCtx.ProfileName); err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return errors.Wrap(err, errUpdateConfig)
	}
	return nil
}

func inferOrganization(ctx context.Context, upCtx *upbound.Context) (string, error) {
	conf, err := upCtx.BuildSDKConfig()
	if err != nil {
		return "", err
	}
	orgs, err := organizations.NewClient(conf).List(ctx)
	if err != nil {
		return "", err
	}
	if len(orgs) == 0 {
		return "", errors.Errorf("You must create an organization to use Upbound. Visit https://accounts.%s to create one.", upCtx.Domain.Host) //nolint:revive // Intentionally human-friendly error.
	}

	// Use the first org in the list as the default. The user can access other
	// orgs later with `up ctx`.
	return orgs[0].Name, nil
}

// constructAuth constructs the body of an Upbound Cloud authentication request
// given the provided credentials.
func constructAuth(username, token, password string) (*auth, profile.TokenType, error) {
	id, profType, err := parseID(username, token)
	if err != nil {
		return nil, "", err
	}
	if profType == profile.TokenTypePAT || profType == profile.TokenTypeRobot {
		password = token
	}
	return &auth{
		ID:       id,
		Password: password,
		Remember: true,
	}, profType, nil
}

// parseID extracts a user ID from a provided token, if available; otherwise, it
// returns the given username. It determines the token type based on the subject
// prefix and returns an appropriate profile.TokenType.
func parseID(user, token string) (string, profile.TokenType, error) {
	if token != "" {
		p := jwt.Parser{}
		claims := &jwt.StandardClaims{}
		_, _, err := p.ParseUnverified(token, claims)
		if err != nil {
			return "", "", err
		}
		if claims.Id == "" {
			return "", "", errors.New(errNoIDInToken)
		}
		if claims.Subject == "" {
			return "", "", errors.New(errNoSubInToken)
		}
		switch {
		case strings.HasPrefix(claims.Subject, "robot|"):
			return claims.Id, profile.TokenTypeRobot, nil
		default:
			return claims.Id, profile.TokenTypePAT, nil
		}
	}
	return user, profile.TokenTypeUser, nil
}

// extractSession extracts the specified cookie from an HTTP response. The
// caller is responsible for closing the response body.
func extractSession(res *http.Response, cookieName string) (string, error) {
	for _, cook := range res.Cookies() {
		if cook.Name == cookieName {
			return cook.Value, nil
		}
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", errors.Wrap(err, errReadBody)
	}
	return "", errors.Errorf(errParseCookieFmt, string(b))
}

// isEmail determines if the specified username is an email address.
func isEmail(user string) bool {
	return strings.Contains(user, "@")
}

func getEndpoint(account url.URL, api url.URL, local string) string {
	totp := local
	if local == "" {
		t := account
		t.Path = totpDisplay
		totp = t.String()
	}
	issueEP := api
	issueEP.Path = issueEndpoint
	issueEP.RawQuery = url.Values{
		"returnTo": []string{totp},
	}.Encode()

	loginEP := account
	loginEP.Path = webLogin
	loginEP.RawQuery = url.Values{
		"returnTo": []string{issueEP.String()},
	}.Encode()
	return loginEP.String()
}

func (c *LoginCmd) exchangeTokenForSession(ctx context.Context, upCtx *upbound.Context, t string) error {
	if t == "" {
		return errors.New("failed to receive callback from web login")
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	e := *upCtx.APIEndpoint
	e.Path = exchangeEndpoint
	e.RawQuery = url.Values{
		"totp": []string{t},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close() //nolint:errcheck // Can't do anything useful with this error.

	user := make(map[string]interface{})
	if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
		return err
	}
	username, ok := user["username"].(string)
	if !ok {
		return errors.New("failed to get user details, code may have expired")
	}
	return setSession(ctx, upCtx, res, profile.TokenTypeUser, username, "")
}

type callbackServer struct {
	token    chan string
	redirect chan string
	port     int
	srv      *http.Server
}

func (cb *callbackServer) getResponse(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()["totp"]
	token := ""
	if len(v) == 1 {
		token = v[0]
	}

	// send the token
	cb.token <- token

	// wait for success or failure redirect
	rd := <-cb.redirect

	http.Redirect(w, r, rd, http.StatusSeeOther)
}

func (cb *callbackServer) shutdownServer(ctx context.Context) error {
	return cb.srv.Shutdown(ctx)
}

func (cb *callbackServer) startServer() (err error) {
	cb.port, err = cb.getPort()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", cb.getResponse)
	cb.srv = &http.Server{
		Handler:           mux,
		Addr:              fmt.Sprintf(":%d", cb.port),
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	go cb.srv.ListenAndServe() //nolint:errcheck // Intentionally not waiting for this to return.

	return nil
}

func (cb *callbackServer) getPort() (int, error) {
	// Create a new server without specifying a port
	// which will result in an open port being chosen
	server, err := net.Listen("tcp", "localhost:0")
	// If there's an error it likely means no ports
	// are available or something else prevented finding
	// an open port
	if err != nil {
		return 0, err
	}
	defer server.Close() //nolint:errcheck // Can't do anything useful with this error.

	// Split the host from the port
	_, portString, err := net.SplitHostPort(server.Addr().String())
	if err != nil {
		return 0, err
	}

	// Return the port as an int
	return strconv.Atoi(portString)
}

func (c *LoginCmd) simpleAuth(ctx context.Context, upCtx *upbound.Context) error {
	if c.Token == "-" {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.Token = strings.TrimSpace(string(b))
	}
	if c.Password == "-" {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.Password = strings.TrimSpace(string(b))
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	userID, profType, err := parseID(c.Username, c.Token)
	if err != nil {
		return err
	}

	var (
		auth *auth
		res  *http.Response
	)

	auth, _, err = constructAuth(userID, c.Token, c.Password)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}

	// Skip API request for robot tokens
	if profType != profile.TokenTypeRobot {
		jsonStr, err := json.Marshal(auth)
		if err != nil {
			return errors.Wrap(err, errLoginFailed)
		}

		if upCtx.APIEndpoint == nil {
			return errors.Wrap(errors.New("API endpoint is not set"), errLoginFailed)
		}

		loginEndpoint := *upCtx.APIEndpoint
		loginEndpoint.Path = loginPath

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginEndpoint.String(), bytes.NewReader(jsonStr))
		if err != nil {
			return errors.Wrap(err, errLoginFailed)
		}

		req.Header.Set("Content-Type", "application/json")

		res, err = c.client.Do(req)
		if err != nil {
			return errors.Wrap(err, errLoginFailed)
		}
		defer res.Body.Close() //nolint:errcheck // Can't do anything useful with this error.
	}
	return errors.Wrap(setSession(ctx, upCtx, res, profType, auth.ID, c.Token), errLoginFailed)
}

func (c *LoginCmd) handleDeviceLogin(upCtx *upbound.Context, token chan<- string, p pterm.TextPrinter) error {
	ep := getEndpoint(*upCtx.AccountsEndpoint, *upCtx.APIEndpoint, "")
	qrterminal.Generate(ep, qrterminal.L, os.Stdout)
	p.Println("Please go to", ep, "and then enter code")
	// TODO(nullable-eth): Add a prompter with timeout?  Difficult to know when they actually
	// finished login to know when the TOTP would expire
	t, err := c.prompter.Prompt("Code", false)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	token <- t
	return nil
}
