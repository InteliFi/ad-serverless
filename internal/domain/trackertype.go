// Package domain — constantes de tipo de tracker.
//
// Portado de: TrackerType (ad-commons v1.4.4)
package domain

const (
	// TrackerPixel é o tracker renderizado como tag <img> (pixel tracking).
	TrackerPixel TrackerType = "PIXEL"

	// TrackerScript é o tracker renderizado como tag <script>.
	TrackerScript TrackerType = "SCRIPT"
)

// TrackerType define como o tracker é renderizado no template HTML.
// PIXEL gera uma tag <img>; SCRIPT gera uma tag <script>.
//
// Portado de: br.com.intv.adserver.business.entity.TrackerType (ad-commons v1.4.4)
type TrackerType string
