// Package domain — structs de campanha e frequência.
//
// Portado de: Campaign.java, FrequencyCap.java (ad-commons v1.4.4)
package domain

import "time"

// Campanha representa uma campanha publicitária veiculada nos hotspots.
// Tabela MySQL: campaigns (~90 registros).
//
// Portado de: br.com.intv.adserver.business.entity.Campaign (ad-commons v1.4.4)
type Campanha struct {
	// id — coluna: id (INT PK AUTO)
	ID int `db:"id"`

	// nome — coluna: name (VARCHAR(50))
	Nome string `db:"name"`

	// ativa — coluna: enabled (TINYINT(1), default 0). Flag de ativação da campanha.
	Ativa bool `db:"enabled"`

	// inicio — coluna: start_date (DATE). Início da janela de veiculação.
	Inicio time.Time `db:"start_date"`

	// fim — coluna: end_date (DATE). Fim da janela de veiculação.
	Fim time.Time `db:"end_date"`

	// hora_cap — coluna: hour_cap (VARCHAR(50)). Ex.: "0;5>10;15>>".
	// Define em quais horas a campanha pode ser veiculada.
	HoraCap string `db:"hour_cap"`

	// dia_semana_cap — coluna: weekday_cap (VARCHAR(50)). Ex.: "1;2>5" (1=SEG…7=DOM).
	DiaSemanaCap string `db:"weekday_cap"`

	// evento_cap — coluna: event_cap (VARCHAR(50)). Reservado, sem lógica no legado.
	EventoCap string `db:"event_cap"`

	// evento_cap_limite — coluna: event_cap_limit (INT(11)). Reservado, sem lógica no legado.
	EventoCapLimite int64 `db:"event_cap_limit"`

	// evento_cap_horas_limite — coluna: event_cap_hours_limit (INT(5)). Reservado, sem lógica no legado.
	EventoCapHorasLimite int `db:"event_cap_hours_limit"`

	// cpe — coluna: cpe (DECIMAL(15,2), default 0.00). Custo por evento.
	CPE float64 `db:"cpe"`

	// campanha_deal — coluna: campaign_deal (DECIMAL(15,2), default 0.00). Valor do contrato.
	CampanhaDeal float64 `db:"campaign_deal"`

	// anunciante — coluna: advertiser (VARCHAR(100))
	Anunciante string `db:"advertiser"`

	// agencia_id — coluna: agency_id (INT FK→agencies)
	AgenciaID int `db:"agency_id"`

	// frequencia_cap — coluna: frequency_cap (INT NULL). Adicionado na V10.
	FrequenciaCap *int `db:"frequency_cap"`

	// Criativos — relacionamento 1:N com creatives (carregado pelas queries do hot path, sem ORM)
	Criativos []Criativo
}
