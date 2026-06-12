package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// TestTratarHello valida o contrato de resposta do hello-handler (M0-02):
// 200, Content-Type application/json e o corpo exato
// {"service":"ad-serverless","stage":"<STAGE>","status":"UP"} — é o mesmo
// formato que o curl do critério de aceite verifica após o deploy.
func TestTratarHello(t *testing.T) {
	// O handler lê o stage da env var STAGE (injetada pelo serverless.yml).
	t.Setenv("STAGE", "dev")

	resp, err := tratarHello(context.Background(), events.APIGatewayV2HTTPRequest{})
	if err != nil {
		t.Fatalf("tratarHello retornou erro inesperado: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, esperado 200", resp.StatusCode)
	}
	if ct := resp.Headers["Content-Type"]; ct != "application/json" {
		t.Errorf("Content-Type = %q, esperado application/json", ct)
	}

	// Compara campo a campo (não a string crua) para o teste não quebrar
	// por detalhe de serialização irrelevante ao contrato.
	var corpo respostaHello
	if err := json.Unmarshal([]byte(resp.Body), &corpo); err != nil {
		t.Fatalf("corpo não é JSON válido: %v (corpo: %s)", err, resp.Body)
	}
	esperado := respostaHello{Service: "ad-serverless", Stage: "dev", Status: "UP"}
	if corpo != esperado {
		t.Errorf("corpo = %+v, esperado %+v", corpo, esperado)
	}
}
