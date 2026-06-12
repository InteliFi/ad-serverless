package domain

import "testing"

// TestEventTypeValoresPersistidos valida que as 17 constantes de EventType usam os valores
// corretos persistidos no banco — incluindo os divergentes (nome Java ≠ valor).
func TestEventTypeValoresPersistidos(t *testing.T) {
	casos := map[EventType]string{
		EventPageView:               "PAGE_VIEW",
		EventImpressionPreRoll:      "IMPRESSION_PRE_ROLL",
		EventClickPreRoll:           "CLICK_PRE_ROLL",
		EventImpressionCampaign:     "IMPRESSION_CAMPAIGN",
		EventClickCampaign:          "CLICK_CAMPAIGN",
		EventVideoStarted:           "VIDEO_STARTED",
		EventPlayed25Per:            "25_PER_PLAYED", // ⚠️ diverge do nome Java PLAYED_25_PER
		EventPlayed50Per:            "50_PER_PLAYED", // ⚠️ diverge do nome Java PLAYED_50_PER
		EventPlayed75Per:            "75_PER_PLAYED", // ⚠️ diverge do nome Java PLAYED_75_PER
		EventVideoEnd:               "VIDEO_ENDED",   // ⚠️ diverge do nome Java VIDEO_END
		EventTrackingPixel:          "TRACKING_PIXEL",
		EventRedirect:               "REDIRECT_CAMPAIGN", // ⚠️ diverge do nome Java REDIRECT
		EventPostbackClick:          "POSTBACK_CLICK",
		EventPostbackCPL:            "POSTBACK_CPL",
		EventPostbackCPA:            "POSTBACK_CPA",
		EventPostbackInstallAndroid: "POSTBACK_INSTALL_ANDROID",
		EventPostbackInstallIOS:     "POSTBACK_INSTALL_IOS",
	}

	if len(casos) != 17 {
		t.Fatalf("esperado 17 tipos de evento, got %d", len(casos))
	}

	for evt, esperado := range casos {
		if string(evt) != esperado {
			t.Errorf("EventType %q: valor persistido = %q, esperado %q", evt, evt, esperado)
		}
	}
}

// TestEventTypeValido valida o helper de validação.
func TestEventTypeValido(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado bool
	}{
		{"PAGE_VIEW", true},
		{"25_PER_PLAYED", true},
		{"REDIRECT_CAMPAIGN", true},
		{"VIDEO_ENDED", true},
		{"POSTBACK_INSTALL_IOS", true},
		{"INVALIDO", false},
		{"", false},
		// Nomes Java que NÃO são valores persistidos:
		{"PLAYED_25_PER", false}, // nome Java, não é o valor no banco
		{"VIDEO_END", false},     // nome Java, não é o valor no banco
		{"REDIRECT", false},      // nome Java, não é o valor no banco
	}

	for _, c := range casos {
		if got := EventTypeValido(c.entrada); got != c.esperado {
			t.Errorf("EventTypeValido(%q) = %v, esperado %v", c.entrada, got, c.esperado)
		}
	}
}

// TestCreativeTypeContagem valida que os tipos de criativo estão definidos.
// O Java tem 45 valores (a spec dizia 46, mas a fonte real é o enum CreativeType.Values).
func TestCreativeTypeContagem(t *testing.T) {
	if len(creativeTypeValidos) != 45 {
		t.Errorf("esperado 45 CreativeTypes, got %d", len(creativeTypeValidos))
	}
}

// TestCreativeTypeValores valida alguns valores representativos.
func TestCreativeTypeValores(t *testing.T) {
	casos := map[CreativeType]struct{}{
		CreativeBanner:                        {},
		CreativeVideo:                         {},
		CreativeVast:                          {},
		CreativeProgramaticaClaroPrerollClick: {},
		CreativePixelTrackingSerasa:           {},
		CreativeUndef:                         {},
	}

	for ct := range casos {
		if !CreativeTypeValido(string(ct)) {
			t.Errorf("CreativeType %q não foi reconhecido como válido", ct)
		}
	}
}

// TestContentTypeCodigos valida os 7 tipos de conteúdo e seus códigos.
func TestContentTypeCodigos(t *testing.T) {
	casos := map[ContentType]string{
		ContentBannerPreRollMobile:         "BPRM",
		ContentBannerCampaignMobile:        "BCM",
		ContentBackgroundMobile:            "BGM",
		ContentBannerPreRollTabletDesktop:  "BPRTD",
		ContentBannerCampaignTabletDesktop: "BCTD",
		ContentBackgroundTabletDesktop:     "BGTD",
		ContentViewideo:                    "VID",
	}

	if len(casos) != 7 {
		t.Fatalf("esperado 7 ContentTypes, got %d", len(casos))
	}

	for ct, codigoEsperado := range casos {
		codigo := ct.ConteTypeCodigo()
		if codigo != codigoEsperado {
			t.Errorf("ContentType(%q).ConteTypeCodigo() = %q, esperado %q", ct, codigo, codigoEsperado)
		}
	}
}

// TestTrackerTypeValores valida os 2 tipos de tracker.
func TestTrackerTypeValores(t *testing.T) {
	if TrackerPixel != "PIXEL" {
		t.Errorf("TrackerPixel = %q, esperado \"PIXEL\"", TrackerPixel)
	}
	if TrackerScript != "SCRIPT" {
		t.Errorf("TrackerScript = %q, esperado \"SCRIPT\"", TrackerScript)
	}
}
