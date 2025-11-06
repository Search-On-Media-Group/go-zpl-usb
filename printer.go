package zplusb

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/google/gousb"
)

type UsbConfig struct {
	Vendor   gousb.ID
	Product  gousb.ID
	Config   int
	Iface    int
	Setup    int
	Endpoint int
}

type UsbZplPrinter struct {
	*gousb.Device
	Config UsbConfig
}

func (printer *UsbZplPrinter) Write(buf []byte) (int, error) {
	intf, done, err := printer.Device.DefaultInterface()

	if err != nil {
		return 0, err
	}
	defer done()

	ep, err := intf.OutEndpoint(printer.Config.Endpoint)
	if err != nil {
		return 0, err
	}

	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l, err := ep.WriteContext(writeCtx, buf)

	if err != nil {
		return l, err
	}

	if l != len(buf) {
		return l, errors.New("partial write")
	}

	return l, nil
}

func GetPrinters(ctx *gousb.Context, config UsbConfig) ([]*UsbZplPrinter, error) {
	var printers []*UsbZplPrinter
	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		var selected = desc.Vendor == config.Vendor
		if config.Product != gousb.ID(0) {
			selected = selected && desc.Product == config.Product
		}
		return selected
	})

	if err != nil {
		return printers, err
	}

	if len(devices) == 0 {
		return printers, ErrorDeviceNotFound
	}

getDevice:
	for _, dev := range devices {
		if runtime.GOOS == "linux" {
			dev.SetAutoDetach(false)
		}

		// get devices with IN direction on endpoint
		for _, cfg := range dev.Desc.Configs {
			for _, alt := range cfg.Interfaces {
				for _, iface := range alt.AltSettings {
					for _, end := range iface.Endpoints {
						if end.Direction == gousb.EndpointDirectionOut {
							config.Config = cfg.Number
							config.Iface = alt.Number
							config.Setup = iface.Number
							config.Endpoint = end.Number
							printer := &UsbZplPrinter{
								dev,
								config,
							}
							// don't timeout reading
							printer.ControlTimeout = 0
							printers = append(printers, printer)
							continue getDevice
						}
					}
				}
			}
		}
	}

	return printers, nil
}
