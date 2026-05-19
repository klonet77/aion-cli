# Flusso di autenticazione — aion-cli

## Panoramica

```
aion-cli          Keycloak          Browser          aion-api
   │                  │                │                 │
   │ 1. genera PKCE   │                │                 │
   │◀────────────────▶│                │                 │
   │                  │                │                 │
   │ 2. server locale │                │                 │
   │   127.0.0.1:9876 │                │                 │
   │                  │                │                 │
   │ 3. apre browser ─────────────────▶│                 │
   │                  │                │                 │
   │                  │◀── GET /auth ──│                 │
   │                  │─── login page─▶│                 │
   │                  │◀── credenziali─│                 │
   │                  │                │                 │
   │◀─────────────────────── 5. redirect ?code=AbCdEf ───│
   │                  │                │                 │
   │ 6. verifica state│                │                 │
   │                  │                │                 │
   │─── 7. POST /token (code + verifier) ──▶│            │
   │◀─── 8. access_token + refresh_token ───│            │
   │                  │                │                 │
   │ 9. salva         │                │                 │
   │  credentials.yaml│                │                 │
   │                  │                │                 │
   │─── Bearer access_token ──────────────────────────▶  │
   │                  │                │  verifica JWKS  │
   │◀─────────────────────────────────────── risposta ── │
```

---

## Passo per passo

### 1. Genera PKCE

Prima di aprire il browser, la CLI genera una coppia di valori crittografici:

```
verifier  = 64 bytes random → codificati base64url
            es. "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

challenge = SHA256(verifier) → codificato base64url
            es. "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
```

Il `verifier` è il segreto — non lascia mai la CLI, non va nel browser, non viaggia in nessun URL. Il `challenge` è pubblico — viene mandato a Keycloak nell'URL di autorizzazione.

---

### 2. Avvia server locale

La CLI avvia un piccolo server HTTP temporaneo su:

```
http://127.0.0.1:9876/callback
```

Questo server esiste solo per ricevere il redirect da Keycloak dopo il login. È in ascolto su `127.0.0.1` (loopback) — non raggiungibile dalla rete.

---

### 3. Apre il browser

La CLI apre il browser con l'URL di autorizzazione di Keycloak:

```
https://argo.datainf.cloud/realms/datainf/protocol/openid-connect/auth
  ?client_id=aion-cli
  &redirect_uri=http://127.0.0.1:9876/callback
  &response_type=code
  &scope=openid profile email
  &code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM
  &code_challenge_method=S256
  &state=97183c2fd15a1dbe637dd1cb98e030bc
```

Parametri chiave:

| Parametro | Valore | Scopo |
|-----------|--------|-------|
| `client_id` | `aion-cli` | identifica il client in Keycloak |
| `redirect_uri` | `http://127.0.0.1:9876/callback` | dove Keycloak manderà il code |
| `response_type` | `code` | vuole un code, non un token diretto |
| `scope` | `openid profile email` | claim da includere nel token |
| `code_challenge` | SHA256(verifier) | challenge PKCE pubblico |
| `code_challenge_method` | `S256` | algoritmo usato per il challenge |
| `state` | valore random | protezione anti-CSRF |

---

### 4. Login su Keycloak

L'utente vede la login page di Keycloak nel browser e inserisce username e password. Keycloak verifica le credenziali e prepara il redirect.

---

### 5. Redirect con code

Dopo il login, Keycloak non manda i token direttamente — manda un **authorization code** temporaneo al server locale della CLI:

```
GET http://127.0.0.1:9876/callback
  ?code=AbCdEf123xyz
  &state=97183c2fd15a1dbe637dd1cb98e030bc
```

Il `code`:
- è monouso — può essere usato una sola volta
- scade in pochi secondi
- da solo è inutile — serve il `verifier` per scambiarlo con un token

---

### 6. Verifica state anti-CSRF

Il server locale verifica che lo `state` ricevuto corrisponda a quello inviato al passo 3. Se non corrisponde, il flusso viene interrotto — qualcuno potrebbe aver tentato un attacco CSRF.

---

### 7. Scambio code → token

La CLI chiama direttamente Keycloak — non tramite browser — con il `code` e il `verifier`:

```http
POST https://argo.datainf.cloud/realms/datainf/protocol/openid-connect/token

Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code
&code=AbCdEf123xyz
&redirect_uri=http://127.0.0.1:9876/callback
&code_verifier=dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk
&client_id=aion-cli
```

Keycloak verifica:

```
SHA256(code_verifier) == code_challenge inviato al passo 3?
   └── SÌ → rilascia i token
   └── NO → rifiuta (code intercettato senza verifier)
```

---

### 8. Token response

Keycloak risponde con i token:

```json
{
  "access_token":       "eyJhbGciOiJSUzI1NiJ9...",
  "refresh_token":      "eyJhbGciOiJIUzUxMiJ9...",
  "expires_in":         300,
  "refresh_expires_in": 1800,
  "token_type":         "Bearer"
}
```

| Token | Algoritmo | Durata | Scopo |
|-------|-----------|--------|-------|
| `access_token` | RS256 (chiave privata Keycloak) | 5 minuti | chiamare aion-api |
| `refresh_token` | HS512 (Keycloak interno) | 30 minuti | ottenere nuovi access token |

---

### 9. Salva credentials.yaml

La CLI salva i token in `~/.config/aion/credentials.yaml` con permessi `0600`:

```yaml
version: 1
current_context: default

contexts:
  default:
    server: https://api.datainf.cloud
    realm: datainf
    client_id: aion-cli

    tokens:
      access_token:      eyJhbGciOiJSUzI1NiJ9...
      refresh_token:     eyJhbGciOiJIUzUxMiJ9...
      expires_at:        2026-05-19T10:05:00Z
      refresh_expires_at: 2026-05-19T10:35:00Z

    user:
      subject:      8cbd9dec-73bc-4728-82f0-fbdd375431f9
      email:        stefano.denardis@datainf.net
      display_name: Stefano De Nardis
      roles:
        - saas-admin
        - grafana-superadmin
```

---

## Chiamata alle API

Dopo il login, ogni chiamata ad aion-api segue questo flusso:

```
CLI legge credentials.yaml
      │
      ├── access_token valido? → usa direttamente
      │
      └── access_token scaduto?
              │
              ├── refresh_token valido?
              │       │
              │       └── POST /token con refresh_token
              │           → nuovo access_token (silenzioso)
              │
              └── refresh_token scaduto?
                      └── "sessione scaduta — esegui aion login"

GET https://api.datainf.cloud/apiv1/...
Authorization: Bearer eyJhbGciOiJSUzI1NiJ9...
```

---

## Come aion-api verifica il token

aion-api non chiama Keycloak per ogni request. Verifica il token localmente con la chiave pubblica RSA:

```
1) Legge kid dall'header JWT
   {"alg":"RS256","kid":"-WWZVRLf8yxZaO8..."}

2) Cerca nel jwksCache (in memoria, TTL 5 min)
   se scaduta → GET https://keycloak/realms/datainf/certs

3) Verifica firma RS256 con chiave pubblica

4) Valida issuer
   iss == "https://argo.datainf.cloud/realms/datainf"

5) Legge ruoli da realm_access.roles
   filtra ruoli di sistema Keycloak

6) Costruisce Principal
   UserID, Subject, Email, Roles
```

Il JWKS endpoint di Keycloak restituisce le chiavi pubbliche:

```json
{
  "keys": [{
    "kid": "-WWZVRLf8yxZaO8EeliC3GXvoCmrHoYjGOE6tc1thII",
    "kty": "RSA",
    "alg": "RS256",
    "use": "sig",
    "n":   "sdfkj3k...",
    "e":   "AQAB"
  }]
}
```

aion-api le cachea per 5 minuti — Keycloak viene contattato raramente, non ad ogni request.

---

## Perché PKCE è sicuro

```
COSA PUÒ VEDERE UN ATTACCANTE     COSA NON PUÒ FARE

code nel redirect URL      →      usarlo senza code_verifier
code_challenge nell'URL    →      ricavare il verifier (SHA256 non è reversibile)
access_token intercettato  →      usarlo dopo 5 minuti (scade)

COSA NON PUÒ MAI VEDERE

code_verifier              →      non lascia mai la CLI
                                  non va nel browser
                                  non appare in nessun URL
                                  viaggia solo in POST diretto a Keycloak
```

---

## Rinnovo automatico del token

Il margine di 30 secondi evita race condition — il token viene rinnovato prima che scada, non dopo:

```
ora                   expires_at
 │                        │
 │◀─── access_token ok ───│
 │                        │
 │         ◀─ 30s ─▶      │
 │              │          │
 │              └── refresh silenzioso
 │                    │
 │                    └── nuovo access_token salvato
```

Se anche il refresh token è scaduto:

```
aion: sessione scaduta — esegui 'aion login'
```

---

## Riferimenti standard

| RFC | Titolo |
|-----|--------|
| RFC 6749 | OAuth 2.0 Authorization Framework |
| RFC 7636 | Proof Key for Code Exchange (PKCE) |
| RFC 7519 | JSON Web Token (JWT) |
| RFC 7517 | JSON Web Key (JWK) — formato JWKS |
| OpenID Connect Core 1.0 | claim `sub`, `email`, `preferred_username` |
