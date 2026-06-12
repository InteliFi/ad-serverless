// Package domain — struct de hotspot.
//
// Portado de: HotSpot.java (ad-commons v1.4.4) + migrations V14–V29
package domain

import "database/sql"

// HotSpot representa um ponto físico de veiculação publicitária (Wi-Fi, quiosque…).
// Tabela MySQL: hotspots (~928 registros).
// A busca por código é sempre em UPPER CASE (chave de cache).
//
// Portado de: br.com.intv.adserver.business.entity.HotSpot (ad-commons v1.4.4)
type HotSpot struct {
	// id — coluna: id (INT PK AUTO)
	ID int `db:"id"`

	// codigo — coluna: code (VARCHAR(100), NOT NULL). Chave de cache, buscado em UPPER CASE.
	Codigo string `db:"code"`

	// descricao — coluna: description (VARCHAR(50))
	Descricao sql.NullString `db:"description"`

	// physical_id — coluna: physical_id (VARCHAR(100)). Identificador físico do equipamento.
	PhysicalID sql.NullString `db:"physical_id"`

	// local_name — coluna: local_name (VARCHAR(255), V19). Nome do local do hotspot.
	LocalName sql.NullString `db:"local_name"`

	// mac_address — coluna: mac_address (VARCHAR(255), V24). Endereço MAC do equipamento.
	MacAddress sql.NullString `db:"mac_address"`

	// data_plan_renew_month_day — coluna: data_plan_renew_month_day (INT, V23). Dia do mês de renovação do plano de dados.
	DataPlanRenewMonthDay sql.NullInt64 `db:"data_plan_renew_month_day"`

	// msp_monthly_fee — coluna: msp_monthly_fee (INT, V28). Mensalidade do provedor Wi-Fi.
	MSPMonthlyFee sql.NullInt64 `db:"msp_monthly_fee"`

	// --- Strings legadas (VARCHAR(100)) ---

	// segmento — coluna: segment (VARCHAR(100)). String legado de segmentação.
	Segmento sql.NullString `db:"segment"`

	// parceiro — coluna: partner (VARCHAR(100)). String legado de parceiro.
	Parceiro sql.NullString `db:"partner"`

	// --- FKs de enriquecimento (V14–V29, maioria NULL) ---

	// segmento_id — coluna: segment_id (INT FK→segments). ID numérico do segmento.
	SegmentoID sql.NullInt64 `db:"segment_id"`

	// parceiro_id — coluna: partner_id (INT FK→partners). ID numérico do parceiro.
	ParceiroID sql.NullInt64 `db:"partner_id"`

	// pais — coluna: country (INT FK→countries). ID do país.
	Pais sql.NullInt64 `db:"country"`

	// msp — coluna: msp (INT FK→msps). ID do provedor Wi-Fi.
	MSP sql.NullInt64 `db:"msp"`

	// cidade — coluna: city (INT FK→cities). ID da cidade.
	Cidade sql.NullInt64 `db:"city"`

	// estado — coluna: state (INT FK→states). ID do estado.
	Estado sql.NullInt64 `db:"state"`

	// ssid — coluna: ssid (INT FK→ssid). ID da rede Wi-Fi.
	SSID sql.NullInt64 `db:"ssid"`

	// operador_id — coluna: operator_id (INT FK→operator). ID do operador de telecomunicações.
	OperadorID sql.NullInt64 `db:"operator_id"`

	// plano_dados — coluna: data_plan (INT FK→data_plan). ID do plano de dados.
	PlanoDados sql.NullInt64 `db:"data_plan"`

	// modem — coluna: modem (INT FK→modem, V25). ID do modelo de modem.
	Modem sql.NullInt64 `db:"modem"`

	// fabricante — coluna: manufacture (INT FK→manufacture, V26). ID do fabricante.
	Fabricante sql.NullInt64 `db:"manufacture"`

	// operadora — coluna: carrier (INT FK→carrier, V27). ID da operadora.
	Operadora sql.NullInt64 `db:"carrier"`

	// moeda_msp — coluna: msp_fee_currency (INT FK→msp_fee_currency, V28). Moeda da mensalidade MSP.
	MoedaMSP sql.NullInt64 `db:"msp_fee_currency"`

	// so — coluna: os (INT FK→os, V29). ID do sistema operativo do hotspot.
	SO sql.NullInt64 `db:"os"`
}
