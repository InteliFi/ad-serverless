// Package domain — struct de tracking pixel.
//
// Portado de: TrackingPixel.java (ad-commons v1.4.4)
package domain

// TrackingPixel é um pixel de rastreamento vinculado a uma campanha.
// Tabela MySQL: tracking_pixels(id, campaign_id FK, url VARCHAR(200)).
//
// Portado de: br.com.intv.adserver.business.entity.TrackingPixel (ad-commons v1.4.4)
type TrackingPixel struct {
	// id — coluna: id (INT PK AUTO)
	ID int `db:"id"`

	// campanha_id — coluna: campaign_id (INT FK→campaigns)
	CampanhaID int `db:"campaign_id"`

	// url — coluna: url (VARCHAR(200)). URL do pixel de tracking.
	URL string `db:"url"`
}
