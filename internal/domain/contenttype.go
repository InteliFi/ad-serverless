// Package domain — constantes de tipo de conteúdo.
//
// Portado de: ContentType.Values (ad-commons v1.4.4)
package domain

const (
	// ContentBannerPreRollMobile é banner de pré-rolagem para mobile. Código no banco: "BPRM".
	ContentBannerPreRollMobile ContentType = "BANNER_PRE_ROLL_MOBILE"

	// ContentBannerCampaignMobile é banner da campanha para mobile. Código no banco: "BCM".
	ContentBannerCampaignMobile ContentType = "BANNER_CAMPAIGN_MOBILE"

	// ContentBackgroundMobile é imagem de fundo para mobile. Código no banco: "BGM".
	ContentBackgroundMobile ContentType = "BACKGROUND_MOBILE"

	// ContentBannerPreRollTabletDesktop é banner de pré-rolagem para tablet e desktop. Código no banco: "BPRTD".
	ContentBannerPreRollTabletDesktop ContentType = "BANNER_PRE_ROLL_TABLET_DESKTOP"

	// ContentBannerCampaignTabletDesktop é banner da campanha para tablet e desktop. Código no banco: "BCTD".
	ContentBannerCampaignTabletDesktop ContentType = "BANNER_CAMPAIGN_TABLET_DESKTOP"

	// ContentBackgroundTabletDesktop é imagem de fundo para tablet e desktop. Código no banco: "BGTD".
	ContentBackgroundTabletDesktop ContentType = "BACKGROUND_TABLET_DESKTOP"

	// ContentViewideo é vídeo. Código no banco: "VID".
	ContentViewideo ContentType = "VIDEO"
)

// contentTypeCode mapeia cada tipo de conteúdo ao código armazenado na tabela content_types.
var contentTypeCode = map[ContentType]string{
	ContentBannerPreRollMobile:         "BPRM",
	ContentBannerCampaignMobile:        "BCM",
	ContentBackgroundMobile:            "BGM",
	ContentBannerPreRollTabletDesktop:  "BPRTD",
	ContentBannerCampaignTabletDesktop: "BCTD",
	ContentBackgroundTabletDesktop:     "BGTD",
	ContentViewideo:                    "VID",
}

// ContentType representa o nome do tipo de conteúdo, conforme a tabela content_types.
// Cada tipo tem um código curto armazenado na coluna "code" da tabela lookup.
//
// Portado de: br.com.intv.adserver.business.entity.ContentType.Values (ad-commons v1.4.4)
type ContentType string

// ConteTypeCodigo retorna o código curto armazenado no banco para este tipo de conteúdo.
func (c ContentType) ConteTypeCodigo() string {
	if code, ok := contentTypeCode[c]; ok {
		return code
	}
	return ""
}
