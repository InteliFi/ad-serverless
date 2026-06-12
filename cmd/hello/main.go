// Package main implementa o hello-handler do ad-serverless.
//
// PROVISÓRIO: removido na issue do ad-handler (M4-03). Este handler existe
// apenas para validar a fundação de deploy criada na issue M0-02:
// serverless.yml (runtime provided.al2023 em arm64, stages dev/prod com
// região própria), HTTP API com payload v2.0 (ADR-001) e o empacotamento
// bootstrap+zip do Makefile. Nenhuma rota de produção depende dele.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	// tzdata embarcado no binário: as Lambdas provided.al2023 não trazem
	// o banco de timezones do SO, e o projeto SEMPRE trabalha datas em
	// America/Sao_Paulo (convenção do CLAUDE.md para todos os mains).
	_ "time/tzdata"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// respostaHello é o corpo JSON devolvido por GET /hello. O formato espelha
// o health do legado ({"version":..., "status":"UP"} — docs/legado/
// 01-endpoints-http.md §1) para já exercitar o contrato de resposta JSON
// que os handlers reais usarão.
type respostaHello struct {
	// Service identifica o serviço — fixo em "ad-serverless".
	Service string `json:"service"`
	// Stage é o stage do deploy (dev/prod), lido da env var STAGE
	// definida no serverless.yml.
	Stage string `json:"stage"`
	// Status é fixo em "UP" — mesmo valor do health legado.
	Status string `json:"status"`
}

// tratarHello responde GET /hello com 200 e o JSON de status do serviço.
// Recebe o request no formato payload v2.0 do API Gateway HTTP API e
// devolve a resposta no mesmo contrato (ADR-001) — é exatamente esse
// roundtrip que a issue M0-02 precisa validar de ponta a ponta.
func tratarHello(_ context.Context, _ events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	corpo, err := json.Marshal(respostaHello{
		Service: "ad-serverless",
		Stage:   os.Getenv("STAGE"),
		Status:  "UP",
	})
	if err != nil {
		// Nunca deve acontecer (struct fixa), mas o contrato do projeto é
		// embrulhar erros com contexto, jamais panic em handler.
		return events.APIGatewayV2HTTPResponse{}, fmt.Errorf("serializar resposta do hello: %w", err)
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(corpo),
	}, nil
}

// main registra o handler no runtime da Lambda. Compilado com
// -tags lambda.norpc (ver Makefile), usa somente o modo de execução
// nativo do provided.al2023.
func main() {
	lambda.Start(tratarHello)
}
