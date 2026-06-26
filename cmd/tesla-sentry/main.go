// Command tesla-sentry toggles Tesla Sentry Mode via the Fleet API.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"tesla-sentry/internal/config"
	"tesla-sentry/internal/keys"
	"tesla-sentry/internal/oauth"
	"tesla-sentry/internal/tesla"
)

const commandTimeout = 3 * time.Minute // wake can take a while

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: tesla-sentry <keygen|register|login|on|off|status|afterblow [minutes] [vent]>")
}

func run(cmd string, args []string) error {
	switch cmd {
	case "keygen":
		return cmdKeygen()
	case "register":
		return cmdRegister()
	case "login":
		return cmdLogin()
	case "on":
		return cmdSet(true)
	case "off":
		return cmdSet(false)
	case "status":
		return cmdStatus()
	case "afterblow":
		return cmdAfterBlow(args)
	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func cmdKeygen() error {
	priv, err := config.Path("private-key.pem")
	if err != nil {
		return err
	}
	pub, err := config.Path("public-key.pem")
	if err != nil {
		return err
	}
	if err := keys.Generate(priv, pub); err != nil {
		return err
	}
	fmt.Printf("Private key: %s\nPublic key:  %s\n", priv, pub)
	fmt.Println("Host the PUBLIC key at: https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem")
	return nil
}

func cmdRegister() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config (run setup first): %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := oauth.NA()
	pt, err := e.PartnerToken(ctx, cfg.ClientID, cfg.ClientSecret, "openid offline_access vehicle_device_data vehicle_cmds")
	if err != nil {
		return fmt.Errorf("partner token: %w", err)
	}
	if err := tesla.RegisterPartner(ctx, pt.AccessToken, cfg.Domain); err != nil {
		return err
	}
	fmt.Printf("Registered domain %s with Tesla.\n", cfg.Domain)
	return nil
}

func cmdLogin() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config (run setup first): %w", err)
	}
	e := oauth.NA()
	redirect := "https://" + cfg.Domain + "/callback"
	authURL := e.AuthorizeURL(cfg.ClientID, redirect, "openid offline_access vehicle_device_data vehicle_cmds", "tesla-sentry")
	fmt.Println("1. Open this URL in a browser and approve:")
	fmt.Println("   " + authURL)
	fmt.Println("2. After redirect, copy the `code` query parameter from the URL bar.")
	fmt.Print("Paste code: ")
	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return fmt.Errorf("read code: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tr, err := e.Exchange(ctx, cfg.ClientID, cfg.ClientSecret, code, redirect)
	if err != nil {
		return err
	}
	tok := &config.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Unix() + tr.ExpiresIn,
	}
	if err := tok.Save(); err != nil {
		return err
	}
	fmt.Println("Login complete. Refresh token saved.")
	fmt.Println("Final setup step: on your phone, open https://tesla.com/_ak/" + cfg.Domain + " and add the virtual key in the Tesla app.")
	return nil
}

// loadForCommand returns config + a fresh access token, persisting any rotation.
func loadForCommand(ctx context.Context) (*config.Config, string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, "", fmt.Errorf("load config: %w", err)
	}
	tok, err := config.LoadToken()
	if err != nil {
		return nil, "", fmt.Errorf("load token (run `login` first): %w", err)
	}
	at, err := tesla.ValidAccessToken(ctx, oauth.NA(), cfg, tok, time.Now().Unix(), func(nt *config.Token) error { return nt.Save() })
	if err != nil {
		return nil, "", fmt.Errorf("refresh token: %w", err)
	}
	return cfg, at, nil
}

func cmdSet(on bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cfg, at, err := loadForCommand(ctx)
	if err != nil {
		return err
	}
	priv, err := config.Path("private-key.pem")
	if err != nil {
		return err
	}
	if err := tesla.SetSentry(ctx, at, cfg.VIN, priv, on); err != nil {
		return err
	}
	state := "off"
	if on {
		state = "on"
	}
	log.Printf("sentry mode set to %s", state)
	return nil
}

const defaultAfterBlowMinutes = 8

// cmdAfterBlow runs the evaporator dry cycle.
// Args (any order): a positive integer = minutes; the word "vent" = also crack
// the windows during the cycle. Defaults: 8 minutes, windows closed.
func cmdAfterBlow(args []string) error {
	opts := tesla.AfterBlowOptions{Duration: defaultAfterBlowMinutes * time.Minute}
	for _, a := range args {
		if a == "vent" {
			opts.VentWindows = true
			continue
		}
		m, err := strconv.Atoi(a)
		if err != nil || m <= 0 {
			return fmt.Errorf("invalid arg %q (expected minutes or \"vent\")", a)
		}
		opts.Duration = time.Duration(m) * time.Minute
	}

	// Timeout must cover the hold duration plus two wake/connect round-trips.
	ctx, cancel := context.WithTimeout(context.Background(), opts.Duration+commandTimeout)
	defer cancel()
	cfg, at, err := loadForCommand(ctx)
	if err != nil {
		return err
	}
	priv, err := config.Path("private-key.pem")
	if err != nil {
		return err
	}
	log.Printf("after-blow: max-defrost for %s (vent=%v)", opts.Duration, opts.VentWindows)
	if err := tesla.AfterBlow(ctx, at, cfg.VIN, priv, opts); err != nil {
		return err
	}
	log.Printf("after-blow: done")
	return nil
}

func cmdStatus() error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cfg, at, err := loadForCommand(ctx)
	if err != nil {
		return err
	}
	on, err := tesla.SentryState(ctx, at, cfg.VIN)
	if err != nil {
		return err
	}
	fmt.Printf("sentry mode: %v\n", on)
	return nil
}
