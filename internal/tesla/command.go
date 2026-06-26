package tesla

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

const userAgent = "tesla-sentry/1.0"

const (
	// onlinePollInterval is how long to wait between vehicle-summary polls while
	// waiting for a wake to complete. Tesla cars typically come online within
	// 15-30s, so 5s catches the transition without hammering the API.
	onlinePollInterval = 5 * time.Second
	// onlinePollMax caps the number of summary polls (~90s ceiling at the above
	// interval). This covers essentially all real wakes while bounding API usage
	// if the car never becomes reachable. The command ctx is the harder bound.
	onlinePollMax = 18
)

// waitOnline polls the lightweight vehicle-summary endpoint until the car
// reports "online". car.Wakeup only guarantees the wake_up request was
// accepted (a sleeping car still answers "asleep"), so without this the signed
// session below races the wake and fails with "vehicle unavailable". Read
// errors and not-yet-online states simply trigger another poll; ctx
// cancellation/timeout is surfaced.
func waitOnline(ctx context.Context, accessToken, vin string) error {
	for attempt := 0; ; attempt++ {
		state, err := VehicleState(ctx, accessToken, vin)
		if err == nil && state == "online" {
			return nil
		}
		if attempt+1 >= onlinePollMax {
			if err != nil {
				return fmt.Errorf("vehicle not online after %d polls: %w", onlinePollMax, err)
			}
			return fmt.Errorf("vehicle not online after %d polls (last state %q)", onlinePollMax, state)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(onlinePollInterval):
		}
	}
}

// withVehicle wakes the vehicle, opens a signed-command session covering all
// subsystems, runs fn, and always disconnects afterward.
func withVehicle(ctx context.Context, accessToken, vin, privateKeyPath string, fn func(*vehicle.Vehicle) error) error {
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

	// Wakeup only confirms the wake_up request was accepted; a sleeping car may
	// still be "asleep" when it returns. Poll the summary endpoint until the car
	// is genuinely online so the signed session below does not race the wake.
	if err := car.Wakeup(ctx); err != nil {
		return fmt.Errorf("wake vehicle: %w", err)
	}
	if err := waitOnline(ctx, accessToken, vin); err != nil {
		return fmt.Errorf("wait online: %w", err)
	}
	if err := car.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	// nil domains = all subsystems (Sentry routes to INFOTAINMENT, climate to
	// the car-server domain).
	if err := car.StartSession(ctx, nil); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	return fn(car)
}

// SetSentry wakes the vehicle and sets Sentry Mode on/off via a signed command.
func SetSentry(ctx context.Context, accessToken, vin, privateKeyPath string, on bool) error {
	return withVehicle(ctx, accessToken, vin, privateKeyPath, func(car *vehicle.Vehicle) error {
		if err := car.SetSentryMode(ctx, on); err != nil {
			return fmt.Errorf("set sentry mode: %w", err)
		}
		return nil
	})
}

// AfterBlowOptions configures the evaporator dry cycle.
type AfterBlowOptions struct {
	Duration    time.Duration // how long to blow before shutting down
	VentWindows bool          // crack the windows during the cycle to expel humid air
}

// AfterBlow dries the A/C evaporator to prevent mold/odor. It runs Max
// Preconditioning ("Max Defrost": maximum heat + high fan) for the configured
// duration, then reverts climate to its prior state and turns it off.
//
// Max Preconditioning is used instead of setting a fixed temperature because
// the vehicle-command SDK can only set the cabin setpoint to Hi/Lo (never a
// specific value), so a plain "set temp to max" would leave the setpoint stuck
// on "Hi". Max Preconditioning is a toggle that reverts cleanly when disabled.
func AfterBlow(ctx context.Context, accessToken, vin, privateKeyPath string, opts AfterBlowOptions) error {
	// Phase 1: start blowing.
	if err := withVehicle(ctx, accessToken, vin, privateKeyPath, func(car *vehicle.Vehicle) error {
		if opts.VentWindows {
			if err := car.VentWindows(ctx); err != nil {
				return fmt.Errorf("vent windows: %w", err)
			}
		}
		if err := car.SetPreconditioningMax(ctx, true, true); err != nil {
			return fmt.Errorf("preconditioning max on: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	// Hold for the dry duration (a fresh signed session is opened for shutdown,
	// so we do not keep a connection alive here).
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(opts.Duration):
	}

	// Phase 2: stop. Best-effort — attempt every undo step even if one fails so
	// we never leave preconditioning running or the windows vented.
	return withVehicle(ctx, accessToken, vin, privateKeyPath, func(car *vehicle.Vehicle) error {
		var errs []error
		if err := car.SetPreconditioningMax(ctx, false, true); err != nil {
			errs = append(errs, fmt.Errorf("preconditioning max off: %w", err))
		}
		if err := car.ClimateOff(ctx); err != nil {
			errs = append(errs, fmt.Errorf("climate off: %w", err))
		}
		if opts.VentWindows {
			if err := car.CloseWindows(ctx); err != nil {
				errs = append(errs, fmt.Errorf("close windows: %w", err))
			}
		}
		return errors.Join(errs...)
	})
}
