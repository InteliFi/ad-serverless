// Package domain — struct de criativo.
//
// Portado de: Creative.java (ad-commons v1.4.4)
package domain

import "database/sql"

// Criativo representa um material publicitário (banner, vídeo, redirect…) vinculado a uma campanha.
// Tabela MySQL: creatives.
//
// Portado de: br.com.intv.adserver.business.entity.Creative (ad-commons v1.4.4)
type Criativo struct {
	// id — coluna: id (INT PK AUTO)
	ID int `db:"id"`

	// campanha_id — coluna: campaign_id (INT FK→campaigns)
	CampanhaID int `db:"campaign_id"`

	// tipo_criativo_id — coluna: creative_type_id (INT FK→creative_types)
	TipoCriativoID int `db:"creative_type_id"`

	// --- URLs deprecated (manter por paridade com o legado, VARCHAR(300)) ---

	// url_portrait — coluna: url_portrait (VARCHAR(300)). Deprecated no legado.
	URLPortrait string `db:"url_portrait"`

	// url_postroll — coluna: url_postroll (VARCHAR(300)). Deprecated no legado.
	URLPostroll string `db:"url_postroll"`

	// url_answer_no — coluna: url_answer_no (VARCHAR(300)). Deprecated no legado.
	URLAnswerNo string `db:"url_answer_no"`

	// url_click — coluna: url_click (VARCHAR(300)). Deprecated no legado.
	URLClick string `db:"url_click"`

	// url_tracking — coluna: url_tracking (VARCHAR(300)). Deprecated no legado.
	URLTracking string `db:"url_tracking"`

	// --- URLs ativas (VARCHAR(200)) ---

	// url_bg — coluna: url_bg (VARCHAR(200)). URL de imagem de fundo (desktop).
	URLBG sql.NullString `db:"url_bg"`

	// url_bg_mobile — coluna: url_bg_mobile (VARCHAR(200)). URL de imagem de fundo (mobile).
	URLBGMobile sql.NullString `db:"url_bg_mobile"`

	// url_preroll — coluna: url_preroll (VARCHAR(200)). Vídeo pré-rolagem (desktop).
	URLPreroll sql.NullString `db:"url_preroll"`

	// url_preroll_mobile — coluna: url_preroll_mobile (VARCHAR(200)). Vídeo pré-rolagem (mobile).
	URLPrerollMobile sql.NullString `db:"url_preroll_mobile"`

	// url_video — coluna: url_video (VARCHAR(200)). URL do vídeo principal (desktop).
	URLVideo sql.NullString `db:"url_video"`

	// url_video_mobile — coluna: url_video_mobile (VARCHAR(200)). URL do vídeo principal (mobile).
	URLVideoMobile sql.NullString `db:"url_video_mobile"`

	// url_banner_campaign — coluna: url_banner_campaign (VARCHAR(200)). Banner da campanha (desktop).
	URLBannerCampaign sql.NullString `db:"url_banner_campaign"`

	// url_banner_campaign_mobile — coluna: url_banner_campaign_mobile (VARCHAR(200)). Banner da campanha (mobile).
	URLBannerCampaignMobile sql.NullString `db:"url_banner_campaign_mobile"`

	// url_redirect — coluna: url_redirect (VARCHAR(200)). URL de redirecionamento (desktop).
	URLRedirect sql.NullString `db:"url_redirect"`

	// url_redirect_mobile — coluna: url_redirect_mobile (VARCHAR(200)). URL de redirecionamento (mobile).
	URLRedirectMobile sql.NullString `db:"url_redirect_mobile"`

	// url_install_google — coluna: url_install_google (VARCHAR(200)). Link Play Store (desktop).
	URLInstallGoogle sql.NullString `db:"url_install_google"`

	// url_install_apple — coluna: url_install_apple (VARCHAR(200)). Link App Store (desktop).
	URLInstallApple sql.NullString `db:"url_install_apple"`

	// url_install_google_mobile — coluna: url_install_google_mobile (VARCHAR(200)). Link Play Store (mobile).
	URLInstallGoogleMobile sql.NullString `db:"url_install_google_mobile"`

	// url_install_apple_mobile — coluna: url_install_apple_mobile (VARCHAR(200)). Link App Store (mobile).
	URLInstallAppleMobile sql.NullString `db:"url_install_apple_mobile"`

	// --- Cores (V12, VARCHAR(7)) ---

	// title_color — coluna: title_color (VARCHAR(7)). Cor do título (desktop), hex #RRGGBB.
	TitleColor sql.NullString `db:"title_color"`

	// title_color_mobile — coluna: title_color_mobile (VARCHAR(7)). Cor do título (mobile).
	TitleColorMobile sql.NullString `db:"title_color_mobile"`

	// button_color — coluna: button_color (VARCHAR(7)). Cor do botão (desktop).
	ButtonColor sql.NullString `db:"button_color"`

	// button_color_mobile — coluna: button_color_mobile (VARCHAR(7)). Cor do botão (mobile).
	ButtonColorMobile sql.NullString `db:"button_color_mobile"`

	// --- Trackers (V12) ---

	// tracker_type — coluna: tracker_type (VARCHAR(50)). Enum PIXEL|SCRIPT.
	TrackerType string `db:"tracker_type"`

	// page_view_tracker — coluna: page_view_tracker (VARCHAR(1024)). URL de tracking de visualização da página.
	PageViewTracker sql.NullString `db:"page_view_tracker"`

	// impression_tracker — coluna: impression_tracker (VARCHAR(1024)). URL de tracking de impressão.
	ImpressionTracker sql.NullString `db:"impression_tracker"`

	// click_campaign_tracker — coluna: click_campaign_tracker (VARCHAR(1024)). URL de tracking de clique na campanha.
	ClickCampaignTracker sql.NullString `db:"click_campaign_tracker"`

	// video_started_tracker — coluna: video_started_tracker (VARCHAR(1024)). URL de tracking de início do vídeo.
	VideoStartedTracker sql.NullString `db:"video_started_tracker"`

	// played_25_per_tracker — coluna: played_25_per_tracker (VARCHAR(1024)). URL de tracking 25% reproduzido.
	Played25PerTracker sql.NullString `db:"played_25_per_tracker"`

	// played_50_per_tracker — coluna: played_50_per_tracker (VARCHAR(1024)). URL de tracking 50% reproduzido.
	Played50PerTracker sql.NullString `db:"played_50_per_tracker"`

	// played_75_per_tracker — coluna: played_75_per_tracker (VARCHAR(1024)). URL de tracking 75% reproduzido.
	Played75PerTracker sql.NullString `db:"played_75_per_tracker"`

	// video_end_tracker — coluna: video_end_tracker (VARCHAR(1024)). URL de tracking fim do vídeo.
	VideoEndTracker sql.NullString `db:"video_end_tracker"`

	// --- Extras (VARCHAR(1024)) ---

	// title_literals — coluna: title_literals (VARCHAR(1024)). Literais para título do template.
	TitleLiterals sql.NullString `db:"title_literals"`

	// prebid_code — coluna: prebid_code (VARCHAR(1024)). Código Prebid para leilão de ads.
	PrebidCode sql.NullString `db:"prebid_code"`
}
