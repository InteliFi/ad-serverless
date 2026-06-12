// Package domain — struct de log de postback (DynamoDB).
//
// Portado de: PostbackLog.java (ad-server v1.4.4)
package domain

// PostbackLog registra um callback recebido de uma rede de afiliados ou plataforma externa.
// Tabela DynamoDB: PostbackLogs.
// PK: transaction_id (S, fallback para click_id), SK: logged_at (S, ISO8601).
type PostbackLog struct {
	// transacao_id — coluna DynamoDB: transaction_id (S). Chave de partição. Fallback para click_id.
	TransacaoID string `dynamodbav:"transaction_id"`

	// registrado_em — coluna DynamoDB: logged_at (S, ISO8601). Chave de ordenação.
	RegistradoEm string `dynamodbav:"logged_at"`

	// campanha_id — coluna DynamoDB: campaign_id (S). ID da campanha como string.
	CampanhaID string `dynamodbav:"campaign_id"`

	// evento — coluna DynamoDB: event (S). Tipo de evento do postback.
	Evento string `dynamodbav:"event"`

	// afiliado_sub — coluna DynamoDB: aff_sub (S). Sub-ID do afiliado que enviou o callback.
	AfiliadoSub string `dynamodbav:"aff_sub"`

	// clique_id — coluna DynamoDB: click_id (S). ID do clique associado ao postback.
	CliqueID string `dynamodbav:"click_id"`

	// origem — coluna DynamoDB: source (S). Origem da rede de afiliados.
	Origem string `dynamodbav:"source"`

	// payout — coluna DynamoDB: payout (N). Valor pago pelo evento.
	Payout float64 `dynamodbav:"payout"`

	// moeda — coluna DynamoDB: currency (S). Moeda do payout.
	Moeda string `dynamodbav:"currency"`

	// valor_venda — coluna DynamoDB: sale_amount (N). Valor da venda reportada.
	ValorVenda float64 `dynamodbav:"sale_amount"`
}
