// Package domain — constantes de tipo de evento.
//
// Portado de: EventType.Values (ad-commons v1.4.4)
package domain

// ⚠️ Os valores persistidos no banco NÃO são iguais aos nomes do enum Java!
// Ex.: PLAYED_25_PER → "25_PER_PLAYED", REDIRECT → "REDIRECT_CAMPAIGN".
// As constantes abaixo usam os VALORES PERSISTIDOS (o que está no banco e DynamoDB).

const (
	// EventPageView é a visualização da página de login/captive portal.
	EventPageView EventType = "PAGE_VIEW"

	// EventImpressionPreRoll é a impressão do vídeo pré-rolagem.
	EventImpressionPreRoll EventType = "IMPRESSION_PRE_ROLL"

	// EventClickPreRoll é o clique no vídeo pré-rolagem.
	EventClickPreRoll EventType = "CLICK_PRE_ROLL"

	// EventImpressionCampaign é a impressão da campanha (banner veiculado).
	EventImpressionCampaign EventType = "IMPRESSION_CAMPAIGN"

	// EventClickCampaign é o clique na campanha.
	EventClickCampaign EventType = "CLICK_CAMPAIGN"

	// EventVideoStarted é o início da reprodução do vídeo.
	EventVideoStarted EventType = "VIDEO_STARTED"

	// EventPlayed25Per é o vídeo reproduzido 25%.
	// ⚠️ nome Java: PLAYED_25_PER, valor persistido: "25_PER_PLAYED".
	EventPlayed25Per EventType = "25_PER_PLAYED"

	// EventPlayed50Per é o vídeo reproduzido 50%.
	// ⚠️ nome Java: PLAYED_50_PER, valor persistido: "50_PER_PLAYED".
	EventPlayed50Per EventType = "50_PER_PLAYED"

	// EventPlayed75Per é o vídeo reproduzido 75%.
	// ⚠️ nome Java: PLAYED_75_PER, valor persistido: "75_PER_PLAYED".
	EventPlayed75Per EventType = "75_PER_PLAYED"

	// EventVideoEnd é o fim da reprodução do vídeo.
	// ⚠️ nome Java: VIDEO_END, valor persistido: "VIDEO_ENDED".
	EventVideoEnd EventType = "VIDEO_ENDED"

	// EventTrackingPixel é o pixel de rastreamento.
	EventTrackingPixel EventType = "TRACKING_PIXEL"

	// EventRedirect é o redirecionamento da campanha.
	// ⚠️ nome Java: REDIRECT, valor persistido: "REDIRECT_CAMPAIGN".
	EventRedirect EventType = "REDIRECT_CAMPAIGN"

	// EventPostbackClick é o postback de clique recebido da rede de afiliados.
	EventPostbackClick EventType = "POSTBACK_CLICK"

	// EventPostbackCPL é o postback CPL (Cost Per Lead) recebido.
	EventPostbackCPL EventType = "POSTBACK_CPL"

	// EventPostbackCPA é o postback CPA (Cost Per Action) recebido.
	EventPostbackCPA EventType = "POSTBACK_CPA"

	// EventPostbackInstallAndroid é o postback de instalação Android recebido.
	EventPostbackInstallAndroid EventType = "POSTBACK_INSTALL_ANDROID"

	// EventPostbackInstallIOS é o postback de instalação iOS recebido.
	EventPostbackInstallIOS EventType = "POSTBACK_INSTALL_IOS"
)

// eventTypeValidos contém todos os valores persistidos para validação rápida.
var eventTypeValidos = map[EventType]struct{}{
	EventPageView:               {},
	EventImpressionPreRoll:      {},
	EventClickPreRoll:           {},
	EventImpressionCampaign:     {},
	EventClickCampaign:          {},
	EventVideoStarted:           {},
	EventPlayed25Per:            {},
	EventPlayed50Per:            {},
	EventPlayed75Per:            {},
	EventVideoEnd:               {},
	EventTrackingPixel:          {},
	EventRedirect:               {},
	EventPostbackClick:          {},
	EventPostbackCPL:            {},
	EventPostbackCPA:            {},
	EventPostbackInstallAndroid: {},
	EventPostbackInstallIOS:     {},
}

// EventType representa o valor string persistido no banco e DynamoDB para um evento de tracking.
// Os valores NÃO são iguais aos nomes do enum Java — ver constantes acima.
//
// Portado de: br.com.intv.adserver.business.entity.EventType.Values (ad-commons v1.4.4)
type EventType string

// EventTypeValido verifica se uma string corresponde a um tipo de evento conhecido.
func EventTypeValido(s string) bool {
	_, ok := eventTypeValidos[EventType(s)]
	return ok
}
