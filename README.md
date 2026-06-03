# async-pix-settlement-api

Backend em Go que simula uma API de liquidacao assincrona estilo PIX. A API recebe uma ordem de transferencia, grava a transacao como `PROCESSING`, publica um evento no Kafka e responde `202 Accepted`. Um worker separado consome o evento e faz a liquidacao no PostgreSQL com lock, transacao SQL e idempotencia.

## Regra dos centavos

Este projeto nao usa `float32` ou `float64` para dinheiro. Todos os valores monetarios sao armazenados e processados como inteiros em centavos:

- R$ 10,50 = `1050`
- R$ 100,00 = `10000`

No PostgreSQL, as colunas sao `BIGINT`: `accounts.balance_cents` e `transactions.amount_cents`. Na aplicacao Go, os valores usam `int64`. O endpoint aceita `amount_cents` diretamente e tambem aceita `amount` decimal como string ou numero JSON, convertendo para centavos sem ponto flutuante.

## Arquitetura

- `cmd/api`: processo HTTP REST.
- `cmd/worker`: processo consumidor Kafka.
- `internal/account`: entidade, repositorio, servico e handler de contas.
- `internal/transaction`: entidade, DTOs, repositorio, servico e handler de transferencias.
- `internal/kafka`: producer e consumer.
- `internal/database`: conexao PostgreSQL.
- `internal/config`: configuracao via variaveis de ambiente e `.env`.
- `migrations`: schema SQL e dados iniciais.

## Fluxo assincrono

1. `POST /transfers` valida campos basicos e existencia das contas.
2. A API cria uma transacao `PROCESSING`.
3. A API publica o evento no topico `pix-transfers`.
4. A API responde imediatamente com `202 Accepted`.
5. O worker consome a mensagem.
6. O worker bloqueia a transacao e as contas com `SELECT ... FOR UPDATE`.
7. O worker verifica saldo, atualiza saldos e marca a transacao como `COMPLETED`.
8. Se nao houver saldo, marca a transacao como `FAILED`.

## Como subir

```bash
docker compose up --build
```

A API fica em:

```bash
http://localhost:8080
```

O Compose tambem sobe PostgreSQL, Zookeeper, Kafka, cria o topico `pix-transfers`, executa a migration inicial e inicia o worker.

## Contas iniciais

As contas criadas pela migration sao:

| Nome | ID | Saldo em centavos |
| --- | --- | ---: |
| Joao | `11111111-1111-1111-1111-111111111111` | `100000` |
| Maria | `22222222-2222-2222-2222-222222222222` | `50000` |
| Carlos | `33333333-3333-3333-3333-333333333333` | `25000` |

## Testar a API

Listar contas:

```bash
curl http://localhost:8080/accounts
```

Criar transferencia:

```bash
curl -X POST http://localhost:8080/transfers \
  -H "Content-Type: application/json" \
  -d '{
    "from_account_id": "11111111-1111-1111-1111-111111111111",
    "to_account_id": "22222222-2222-2222-2222-222222222222",
    "amount_cents": 10050
  }'
```

Tambem e aceito enviar `amount` decimal; a API converte para centavos sem usar floats:

```bash
curl -X POST http://localhost:8080/transfers \
  -H "Content-Type: application/json" \
  -d '{
    "from_account_id": "11111111-1111-1111-1111-111111111111",
    "to_account_id": "22222222-2222-2222-2222-222222222222",
    "amount": "100.50"
  }'
```

Resposta esperada:

```json
{
  "transaction_id": "uuid-gerado",
  "status": "PROCESSING",
  "message": "Transfer accepted for async processing"
}
```

Consultar transacao:

```bash
curl http://localhost:8080/transfers/{transaction_id}
```

Exemplo de resposta depois do worker processar:

```json
{
  "transaction_id": "uuid",
  "from_account_id": "11111111-1111-1111-1111-111111111111",
  "to_account_id": "22222222-2222-2222-2222-222222222222",
  "amount_cents": 10050,
  "status": "COMPLETED"
}
```

## Logs do worker

```bash
docker compose logs -f worker
```

Os logs sao estruturados em JSON e mostram recebimento da mensagem, status final e commit do offset Kafka.

## Consultar o banco

Entrar no `psql`:

```bash
docker compose exec postgres psql -U postgres -d pixdb
```

Consultar contas:

```sql
SELECT id, owner_name, balance_cents FROM accounts ORDER BY owner_name;
```

Consultar transacoes:

```sql
SELECT id, from_account_id, to_account_id, amount_cents, status, created_at, updated_at
FROM transactions
ORDER BY created_at DESC;
```

## Idempotencia

A chave primaria de `transactions.id` e o UUID da transacao. O worker bloqueia a linha da transacao com `FOR UPDATE` e revalida o status dentro da transacao SQL. Se o status ja for `COMPLETED`, a mensagem duplicada e ignorada e o offset e commitado sem debitar novamente.

Essa regra protege contra redelivery do Kafka e contra dois workers tentando processar o mesmo UUID ao mesmo tempo.

## Por que HTTP 202

`202 Accepted` indica que a requisicao foi aceita, mas o processamento final ainda nao terminou. Isso combina com o fluxo de liquidacao assincrona: a API nao debita nem credita durante a chamada HTTP, ela apenas registra a ordem e entrega o evento para o worker.

## Como simula PIX

O projeto modela a separacao entre captura da ordem e liquidacao financeira. A API representa a entrada da ordem de pagamento, Kafka representa a mensageria entre componentes distribuidos, e o worker representa o motor de liquidacao que aplica debito, credito, controle de concorrencia e idempotencia.

## Variaveis de ambiente

Copie o exemplo se quiser sobrescrever configuracoes:

```bash
cp .env.example .env
```

```env
APP_PORT=8080
DATABASE_URL=postgres://postgres:postgres@postgres:5432/pixdb?sslmode=disable
KAFKA_BROKERS=kafka:9092
KAFKA_TOPIC=pix-transfers
```

Para rodar localmente fora do Docker, use `DATABASE_URL` e `KAFKA_BROKERS` apontando para portas expostas no host, por exemplo `localhost:5432` e `localhost:29092`.

## Criterio de aceite

Com `docker compose up --build`, voce consegue listar contas, enviar uma transferencia, receber `202 Accepted`, acompanhar o worker processando, ver os saldos mudarem, consultar a transacao como `COMPLETED` e reprocessar uma mensagem duplicada sem debito duplicado.
