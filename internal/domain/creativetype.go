// Package domain — constantes de tipo de criativo.
//
// Portado de: CreativeType.Values (ad-commons v1.4.4)
package domain

const (
	// CreativeBanner é um banner estático simples.
	CreativeBanner CreativeType = "BANNER"

	// CreativeVideo é um vídeo.
	CreativeVideo CreativeType = "VIDEO"

	// CreativeBannerQuestion é um banner com pergunta (autenticação).
	CreativeBannerQuestion CreativeType = "BANNER_QUESTION"

	// CreativeBannerQuestionNoAuth é um banner com pergunta sem autenticação.
	CreativeBannerQuestionNoAuth CreativeType = "BANNER_QUESTION_NO_AUTH"

	// CreativeAppInstall é instalação de aplicativo.
	CreativeAppInstall CreativeType = "APP_INSTALL"

	// CreativeBannerBusAppInstall é banner para ônibus com instalação de app.
	CreativeBannerBusAppInstall CreativeType = "BANNER_BUS_APP_INSTALL"

	// CreativeBannerButtonClose é banner com botão de fechar.
	CreativeBannerButtonClose CreativeType = "BANNER_BUTTONCLOSE"

	// CreativeJustBanner é apenas banner (sem vídeo).
	CreativeJustBanner CreativeType = "JUST_BANNER"

	// CreativeVast é VAST (Video Ad Serving Template).
	CreativeVast CreativeType = "VAST"

	// CreativeVideoVpaidSpace é vídeo VPAID Space.
	CreativeVideoVpaidSpace CreativeType = "VIDEO_VPAID_SPACE"

	// CreativeBannerSpace é espaço de banner genérico.
	CreativeBannerSpace CreativeType = "BANNER_SPACE"

	// CreativeWiconnectVideo é vídeo WiConnect (MSP Mambo).
	CreativeWiconnectVideo CreativeType = "WICONNECT_VIDEO"

	// CreativeWiconnectBanner é banner WiConnect (MSP Mambo).
	CreativeWiconnectBanner CreativeType = "WICONNECT_BANNER"

	// CreativeSmartadRss é SmartAd RSS (Programática).
	CreativeSmartadRss CreativeType = "SMARTAD_RSS"

	// CreativeGoogleAdUnit é Google Ad Unit.
	CreativeGoogleAdUnit CreativeType = "GOOGLE_AD_UNIT"

	// CreativeRedirectPostback é redirecionamento com postback.
	CreativeRedirectPostback CreativeType = "REDIRECT_POSTBACK"

	// CreativeGamAeroVix é GAM Aeroportuário de Vitória.
	CreativeGamAeroVix CreativeType = "GAM_AERO_VIX"

	// CreativeGamSptransNardelli é GAM SPTrans Nardelli.
	CreativeGamSptransNardelli CreativeType = "GAM_SPTRANS_NARDELLI"

	// CreativeProgramaticaVast é programática VAST.
	CreativeProgramaticaVast CreativeType = "PROGRAMATICA_VAST"

	// CreativeProgramaticaSelfclose é programática com auto-fecha.
	CreativeProgramaticaSelfclose CreativeType = "PROGRAMATICA_SELFCLOSE"

	// CreativeProgramatica é programática genérica.
	CreativeProgramatica CreativeType = "PROGRAMATICA"

	// CreativeProgramaticaSmart é programática Smart.
	CreativeProgramaticaSmart CreativeType = "PROGRAMATICA_SMART"

	// CreativeProgramaticaClaro é programática Claro.
	CreativeProgramaticaClaro CreativeType = "PROGRAMATICA_CLARO"

	// CreativeProgramaticaWebmotors é programática Webmotors.
	CreativeProgramaticaWebmotors CreativeType = "PROGRAMATICA_WEBMOTORS"

	// CreativeProgramaticaBmc é programática BMC.
	CreativeProgramaticaBmc CreativeType = "PROGRAMATICA_BMC"

	// CreativeBannerWebmotors é banner Webmotors.
	CreativeBannerWebmotors CreativeType = "BANNER_WEBMOTORS"

	// CreativeBannerBmc é banner BMC.
	CreativeBannerBmc CreativeType = "BANNER_BMC"

	// CreativeProgramaticaClaroPrerollClick é programática Claro preroll com clique.
	CreativeProgramaticaClaroPrerollClick CreativeType = "PROGRAMATICA_CLARO_PREROLL_CLICK"

	// CreativeCampaignProgramaticaClaroPrerollClick é campanha programática Claro preroll com clique.
	CreativeCampaignProgramaticaClaroPrerollClick CreativeType = "CAMPAIGN_PROGRAMATICA_CLARO_PREROLL_CLICK"

	// CreativeVideoCampaignProgramatica é vídeo campanha programática.
	CreativeVideoCampaignProgramatica CreativeType = "VIDEO_CAMPAIGN_PROGRAMATICA"

	// CreativeBetanoPrezao é Betão Preção (campanha específica).
	CreativeBetanoPrezao CreativeType = "BETANO_PREZAO"

	// CreativeSmartClaroApp é Smart Claro App.
	CreativeSmartClaroApp CreativeType = "SMART_CLARO_APP"

	// CreativeAdforceDisplayBanner é AdForce Display Banner.
	CreativeAdforceDisplayBanner CreativeType = "ADFORCE_DISPLAY_BANNER"

	// CreativeVideoVpaidSpaceWico é vídeo VPAID Space WiCo.
	CreativeVideoVpaidSpaceWico CreativeType = "VIDEO_VPAID_SPACE_WICO"

	// CreativeSmartadSptrans é SmartAd SPTrans.
	CreativeSmartadSptrans CreativeType = "SMARTAD_SPTRANS"

	// CreativeSmartadAerBsb é SmartAd Aeroporto Brasília.
	CreativeSmartadAerBsb CreativeType = "SMARTAD_AER_BSB"

	// CreativeSmartadAerVcp é SmartAd Aeroporto Viracopos.
	CreativeSmartadAerVcp CreativeType = "SMARTAD_AER_VCP"

	// CreativeIma é IMA (Interactive Media Ads).
	CreativeIma CreativeType = "IMA"

	// CreativeImaProgramatica é IMA Programática.
	CreativeImaProgramatica CreativeType = "IMA_PROGRAMATICA"

	// CreativeCampaignProgramatica é campanha programática genérica.
	CreativeCampaignProgramatica CreativeType = "CAMPAIGN_PROGRAMATICA"

	// CreativeNoadBannerProgramatica é NoAd Banner Programática.
	CreativeNoadBannerProgramatica CreativeType = "NOAD_BANNER_PROGRAMATICA"

	// CreativeVast420 é VAST 420 (formato específico).
	CreativeVast420 CreativeType = "VAST420"

	// CreativeSptransBanner é banner SPTrans.
	CreativeSptransBanner CreativeType = "SPTRANS_BANNER"

	// CreativePixelTrackingSerasa é pixel de tracking Serasa.
	CreativePixelTrackingSerasa CreativeType = "PIXEL_TRACKING_SERASA"

	// CreativeUndef é tipo indefinido (fallback).
	CreativeUndef CreativeType = "UNDEF"
)

// creativeTypeValidos contém todos os tipos para validação rápida.
var creativeTypeValidos = map[CreativeType]struct{}{
	CreativeBanner:                                {},
	CreativeVideo:                                 {},
	CreativeBannerQuestion:                        {},
	CreativeBannerQuestionNoAuth:                  {},
	CreativeAppInstall:                            {},
	CreativeBannerBusAppInstall:                   {},
	CreativeBannerButtonClose:                     {},
	CreativeJustBanner:                            {},
	CreativeVast:                                  {},
	CreativeVideoVpaidSpace:                       {},
	CreativeBannerSpace:                           {},
	CreativeWiconnectVideo:                        {},
	CreativeWiconnectBanner:                       {},
	CreativeSmartadRss:                            {},
	CreativeGoogleAdUnit:                          {},
	CreativeRedirectPostback:                      {},
	CreativeGamAeroVix:                            {},
	CreativeGamSptransNardelli:                    {},
	CreativeProgramaticaVast:                      {},
	CreativeProgramaticaSelfclose:                 {},
	CreativeProgramatica:                          {},
	CreativeProgramaticaSmart:                     {},
	CreativeProgramaticaClaro:                     {},
	CreativeProgramaticaWebmotors:                 {},
	CreativeProgramaticaBmc:                       {},
	CreativeBannerWebmotors:                       {},
	CreativeBannerBmc:                             {},
	CreativeProgramaticaClaroPrerollClick:         {},
	CreativeCampaignProgramaticaClaroPrerollClick: {},
	CreativeVideoCampaignProgramatica:             {},
	CreativeBetanoPrezao:                          {},
	CreativeSmartClaroApp:                         {},
	CreativeAdforceDisplayBanner:                  {},
	CreativeVideoVpaidSpaceWico:                   {},
	CreativeSmartadSptrans:                        {},
	CreativeSmartadAerBsb:                         {},
	CreativeSmartadAerVcp:                         {},
	CreativeIma:                                   {},
	CreativeImaProgramatica:                       {},
	CreativeCampaignProgramatica:                  {},
	CreativeNoadBannerProgramatica:                {},
	CreativeVast420:                               {},
	CreativeSptransBanner:                         {},
	CreativePixelTrackingSerasa:                   {},
	CreativeUndef:                                 {},
}

// CreativeType representa o nome do tipo de criativo, conforme a tabela creative_types.
// Cada valor corresponde ao campo "name" da tabela lookup.
//
// Portado de: br.com.intv.adserver.business.entity.CreativeType.Values (ad-commons v1.4.4)
type CreativeType string

// CreativeTypeValido verifica se uma string corresponde a um tipo de criativo conhecido.
func CreativeTypeValido(s string) bool {
	_, ok := creativeTypeValidos[CreativeType(s)]
	return ok
}
