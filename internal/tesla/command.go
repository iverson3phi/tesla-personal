package tesla

import (
	"context"
	"fmt"

	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

const userAgent = "tesla-sentry/1.0"

// SetSentry wakes the vehicle and sets Sentry Mode on/off via a signed command.
func SetSentry(ctx context.Context, accessToken, vin, privateKeyPath string, on bool) error {
	key, err := protocol.LoadPrivateKey(privateKeyPath)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	acct, err := account.New(accessToken, userAgent)
	if err != nil {
		return fmt.Errorf("build account: %w", err)
	}

	car, err := acct.GetVehicle(ctx, vin, key, nil)
	if err != nil {
		return fmt.Errorf("get vehicle: %w", err)
	}
	defer car.Disconnect()

	// Wakeup blocks until the car reports "online" (or ctx expires).
	if err := car.Wakeup(ctx); err != nil {
		return fmt.Errorf("wake vehicle: %w", err)
	}
	if err := car.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	// nil domains = all subsystems; Sentry Mode routes to INFOTAINMENT.
	if err := car.StartSession(ctx, nil); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	if err := car.SetSentryMode(ctx, on); err != nil {
		return fmt.Errorf("set sentry mode: %w", err)
	}
	return nil
}
