// Package domain — structs de tracking (MySQL + DynamoDB).
//
// Portado de: AdTracker.java (ad-commons v1.4.4)
package domain

import "time"

// AdTracker registra um evento de veiculação ou interação no MySQL.
// Tabela MySQL: ad_trackers (~14M linhas, write-heavy).
// O campo hotspot_id é NULL para tracking pixel e postback (não há hotspot físico).
//
// Portado de: br.com.intv.adserver.business.entity.AdTracker (ad-commons v1.4.4)
type AdTracker struct {
	// id — coluna: id (INT PK AUTO)
	ID int `db:"id"`

	// campanha_id — coluna: campaign_id (INT). Referência simples, sem FK ORM no legado.
	CampanhaID int `db:"campaign_id"`

	// hotspot_id — coluna: hotspot_id (VARCHAR(50)). NULL para tracking pixel e postback.
	HotspotID string `db:"hotspot_id"`

	// evento_tipo — coluna: event_type (VARCHAR(50)). Valor string do EventType persistido no banco.
	EventoTipo string `db:"event_type"`

	// data_criacao — coluna: creation_date (DATETIME, default CURRENT_TIMESTAMP). Timestamp exato do evento.
	DataCriacao time.Time `db:"creation_date"`

	// data_evento — coluna: event_date (DATE). Data do evento para agrupamento por dia.
	DataEvento time.Time `db:"event_date"`
}

// AdTrackerEvent é a réplica DynamoDB de um evento de tracking.
// Tabela DynamoDB: AdTrackers (classe STANDARD_INFREQUENT_ACCESS, billing PAY_PER_REQUEST).
// PK: campaign_id (S), SK: created_at_id (S, formato EXATO "<ISO8601>#<rds_id>").
type AdTrackerEvent struct {
	// campanha_id — coluna DynamoDB: campaign_id (S). Chave de partição.
	CampanhaID string `dynamodbav:"campaign_id"`

	// criado_em_id — coluna DynamoDB: created_at_id (S). Chave de ordenação.
	// Formato EXATO: "<ISO8601>#<rds_id>", ex.: "2024-06-10T15:30:45.123Z#12345".
	CriadoEmID string `dynamodbav:"created_at_id"`

	// hotspot_id — coluna DynamoDB: hotspot_id (S). Pode ser vazio para postback/pixel.
	HotspotID string `dynamodbav:"hotspot_id"`

	// evento_tipo — coluna DynamoDB: event_type (S). Valor string do EventType.
	EventoTipo string `dynamodbav:"event_type"`

	// data_evento — coluna DynamoDB: event_date (S, "yyyy-MM-dd" em America/Sao_Paulo).
	DataEvento string `dynamodbav:"event_date"`

	// rds_id — coluna DynamoDB: rds_id (N). ID auto-increment do registro MySQL original.
	RDSID int64 `dynamodbav:"rds_id"`
}
