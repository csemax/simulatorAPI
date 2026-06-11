````md
# DANTE Simulator API

Simulator API adalah service proxy untuk membandingkan request client saat melewati **DANTE Middleware** atau langsung ke **Legacy API**.

## Flow

```txt
Mobile App / Client
        |
        v
Simulator API
        |
        |-- X-Use-Dante: true  -> DANTE Middleware
        |
        |-- X-Use-Dante: false -> Legacy API
````

## Fitur

* Toggle DANTE ON/OFF
* Simulasi network profile:

  * `5g`
  * `4g`
  * `3g`
  * `rural`
  * `unstable`
* Proxy request ke DANTE atau Legacy
* Response metadata untuk demo:

  * target backend
  * network profile
  * response time
  * simulated delay

## Environment

Buat file `.env`:

```env
APP_PORT=8090
APP_ADDR=:8090

DANTE_BASE_URL=https://your-dante-api-url
LEGACY_BASE_URL=https://legacy.litegral.com

HTTP_CLIENT_TIMEOUT_SECONDS=30
```

## Run

### Local

```bash
go mod tidy
go run ./cmd/server
```

### Docker

```bash
docker compose up --build
```

## Endpoint

### Health Check

```http
GET /ready
```

### Network Profiles

```http
GET /profiles
```

### Proxy

```http
ANY /proxy/*
```

### Direct Legacy

Forward langsung ke `LEGACY_BASE_URL` tanpa melewati DANTE Middleware:

```http
ANY /legacy/*
```

Prefix `/legacy` akan dipotong sebelum request dikirim ke legacy backend. Contoh:

```http
POST /legacy/axis2/services/BankService
```

Akan diteruskan ke:

```http
{LEGACY_BASE_URL}/axis2/services/BankService
```

Contoh endpoint:

```http
GET /proxy/v1/merchants/{merchantId}
GET /proxy/v1/transactions/{transactionId}
GET /proxy/v1/transactions/{transactionId}/status
GET /proxy/v1/accounts/{accountId}/transactions
GET /proxy/internal/system/status
POST /legacy/axis2/services/BankService
GET /legacy/axis2/services/BankService?wsdl
```

## Request Header

```http
X-Use-Dante: true
X-Network-Profile: 3g
```

Keterangan:

| Header              | Fungsi                                     |
| ------------------- | ------------------------------------------ |
| `X-Use-Dante`       | `true` ke DANTE, `false` ke Legacy         |
| `X-Network-Profile` | `5g`, `4g`, `3g`, `rural`, atau `unstable` |

## Contoh Request

### DANTE ON

```bash
curl -i \
  -H "X-Use-Dante: true" \
  -H "X-Network-Profile: 3g" \
  http://localhost:8090/proxy/v1/merchants/MRC001
```

### DANTE OFF

```bash
curl -i \
  -H "X-Use-Dante: false" \
  -H "X-Network-Profile: 3g" \
  http://localhost:8090/proxy/v1/merchants/MRC001
```

### Direct Legacy SOAP

```bash
curl -i \
  -H "Content-Type: text/xml; charset=utf-8" \
  -H "SOAPAction: balance" \
  -H "X-Network-Profile: 4g" \
  --data '<?xml version="1.0" encoding="UTF-8"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ns="http://BankService.services.axis2"><soapenv:Header/><soapenv:Body><ns:balance><ns:args0>2623860486223779</ns:args0><ns:args1>123456</ns:args1></ns:balance></soapenv:Body></soapenv:Envelope>' \
  http://localhost:8090/legacy/axis2/services/BankService
```

## Response Header

Simulator API menambahkan header:

```http
X-Simulator-Target: dante
X-Simulator-Network-Profile: 3g
X-Simulator-Use-Dante: true
X-Simulator-Duration-MS: 532
X-Simulator-Simulated-Delay-MS: 421
```

Header ini bisa ditampilkan di mobile app untuk demo perbandingan.

## Troubleshooting

### `docker compose ps` kosong

Jalankan:

```bash
docker compose up --build
```

### `403 proxy_path_not_allowed`

Path belum masuk whitelist.

### `422`

Request sudah sampai ke backend, tapi ID atau format request tidak valid.

### `502`

Simulator API gagal menghubungi DANTE atau Legacy. Cek:

```env
DANTE_BASE_URL=
LEGACY_BASE_URL=
```

```

Kamu bisa langsung salin ini ke `README.md`.
```
