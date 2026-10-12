package service

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

const errBase = "org.nitrosense.Daemon1.Error."

func errNotSupported(what string) *dbus.Error {
	return dbus.NewError(errBase+"NotSupported", []interface{}{what + " is not available on this hardware"})
}

func errNotAllowed(format string, a ...interface{}) *dbus.Error {
	return dbus.NewError(errBase+"NotAllowed", []interface{}{fmt.Sprintf(format, a...)})
}

func errInvalid(format string, a ...interface{}) *dbus.Error {
	return dbus.NewError(errBase+"InvalidArgument", []interface{}{fmt.Sprintf(format, a...)})
}

func errHardware(err error) *dbus.Error {
	return dbus.NewError(errBase+"Hardware", []interface{}{err.Error()})
}

func errDenied(action string) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []interface{}{"not authorized for " + action})
}
