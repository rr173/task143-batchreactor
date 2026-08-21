package store

import "errors"

// Domain errors mapped to HTTP statuses by the httpapi layer. Each is a sentinel
// so the service can wrap it with %w and the handler can errors.Is it.
var (
	ErrNotFound           = errors.New("store: not found")
	ErrConflict           = errors.New("store: conflict")
	ErrStateConflict      = errors.New("store: illegal state transition")
	ErrInvariant          = errors.New("store: invariant violation")
	ErrThermalIncompatible = errors.New("store: recipe not thermally compatible with reactor")
	ErrLowConversion      = errors.New("store: conversion below spec")
	ErrRunaway            = errors.New("store: thermal runaway")
	ErrReactorBusy        = errors.New("store: reactor busy")
	ErrAlreadyPlanned     = errors.New("store: campaign already planned")
	ErrNotPlanned         = errors.New("store: campaign not planned")
)
