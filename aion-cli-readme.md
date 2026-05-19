# aion-cli

> CLI ufficiale per la piattaforma **aion** — autenticazione sicura via browser, gestione credenziali locale, rinnovo automatico dei token.

---

## Indice

- [Panoramica](#panoramica)
- [Architettura](#architettura)
- [Protocolli e standard](#protocolli-e-standard)
  - [OAuth 2.0](#oauth-20)
  - [OpenID Connect (OIDC)](#openid-connect-oidc)
  - [PKCE](#pkce--proof-key-for-code-exchange)
  - [JWT](#jwt--json-web-token)
- [Flusso di autenticazione dettagliato](#flusso-di-autenticazione-dettagliato)
- [Gestione dei token](#gestione-dei-token)
- [Struttura del progetto](#struttura-del-progetto)
- [Configurazione Keycloak](#configurazione-keycloak)
- [Credenziali locali](#credenziali-locali)
- [Comandi](#comandi)
- [Sicurezza](#sicurezza)
- [Setup e build](#setup-e-build)
- [Riferimenti](#riferimenti)

---

## Panoramica

`aion-cli` è la CLI ufficiale per interagire con **aion-api**, la piattaforma di gestione SaaS multi-tenant. Funziona in modo simile a `kubectl` per Kubernetes o `gh` per GitHub:

- **un solo login** — ti autentichi una volta, le credenziali vengono salvate localmente
- **rinnovo silenzioso** — il token viene rinnovato automaticamente senza disturbarti
- **sicurezza moderna** — nessuna password salvata, nessun secret nel codice, solo token temporanei

```
┌─────────────┐     PKCE + browser      ┌─────────────┐
│  aion-cli   │ ─────────────────────▶  │  Keycloak   │
│             │ ◀─────────────────────  │             │
│             │   access + refresh JWT  │             │
└─────────────┘                         └─────────────┘
       │
       │  Bearer JWT
       ▼
┌─────────────┐
│  aion-api   │
└─────────────┘
```

---

## Architettura

### Componenti principali

```
aion-cli
├── cmd/aion/main.go              Entry point del binary
├── commands/                     Comandi CLI (cobra)
│   ├── root.go                   Comando root, registrazione sottocomandi
│   ├── login.go                  aion login
│   ├── logout.go                 aion logout / logout-all
│   └── whoami.go                 aion whoami
└── internal/                     Logica interna (non esposta)
    ├── auth/
    │   ├── pkce.go               Generazione PKCE verifier/challenge
    │   ├── login.go              Flusso browser + server callback locale
    │   └── token.go              Scambio code, refresh, parsing JWT
    ├── config/
    │   └── credentials.go        Read/write ~/.config/aion/credentials.yaml
    └── api/
        └── client.go             Client HTTP con refresh automatico
```

### Dipendenze esterne

| Libreria | Scopo |
|----------|-------|
| `github.com/spf13/cobra` | Framework CLI (stesso usato da kubectl, hugo, gh) |
| `github.com/golang-jwt/jwt/v5` | Parsing e lettura claim dai token JWT |
| `gopkg.in/yaml.v3` | Serializzazione/deserializzazione credentials.yaml |

---

## Protocolli e standard

### OAuth 2.0

**OAuth 2.0** (RFC 6749) è il framework di autorizzazione su cui si basa tutto il sistema. Definisce come un'applicazione può ottenere accesso a risorse per conto di un utente senza che l'utente consegni la propria password all'applicazione.

Il concetto chiave è la **delega**: invece di dare la password a `aion-cli`, l'utente la inserisce direttamente su Keycloak (il server di autorizzazione), che poi rilascia un token all'applicazione.

**I ruoli in OAuth 2.0:**

```
Resource Owner   = l'utente (stefano.denardis)
Client           = aion-cli
Authorization Server = Keycloak
Resource Server  = aion-api
```

**Il flow usato — Authorization Code Flow:**

```
aion-cli                  Keycloak                  aion-api
    │                         │                         │
    │── redirect browser ────▶│                         │
    │                         │◀── utente fa login ─────│
    │◀── redirect + code ─────│                         │
    │                         │                         │
    │── POST /token ──────────▶│                         │
    │   (code + verifier)      │                         │
    │◀── access_token ─────────│                         │
    │    refresh_token         │                         │
    │                         │                         │
    │── GET /api/... ─────────────────────────────────▶ │
    │   Authorization: Bearer <access_token>             │
    │◀── risposta ───────────────────────────────────── │
```

---

### OpenID Connect (OIDC)

**OpenID Connect** (OIDC) è uno strato di identità costruito sopra OAuth 2.0. OAuth 2.0 risolve il problema dell'autorizzazione ("puoi accedere a questa risorsa"), OIDC risolve quello dell'autenticazione ("chi sei").

Con OIDC, il token rilasciato da Keycloak contiene claim standardizzate sull'identità dell'utente:

```json
{
  "sub": "8cbd9dec-73bc-4728-82f0-fbdd375431f9",
  "preferred_username": "stefano.denardis",
  "email": "stefano.denardis@datainf.net",
  "name": "Stefano De Nardis",
  "realm_access": {
    "roles": ["saas-admin", "grafana-superadmin"]
  }
}
```

`aion-cli` usa OIDC per:
- estrarre le informazioni dell'utente (`preferred_username`, `email`, `name`)
- leggere i ruoli applicativi dal token (`realm_access.roles`)
- mostrare queste informazioni con `aion whoami`

Il parametro `scope: "openid profile email"` nella request di autorizzazione istruisce Keycloak a includere queste claim nel token.

---

### PKCE — Proof Key for Code Exchange

**PKCE** (RFC 7636, pronunciato "pixie") è un'estensione del Authorization Code Flow progettata per proteggere le applicazioni pubbliche — quelle che non possono tenere un secret in modo sicuro.

`aion-cli` è un'applicazione pubblica: chiunque può decompilare il binary e leggerne il contenuto. Salvare un `client_secret` nel codice sarebbe inutile. PKCE risolve questo problema con un meccanismo di challenge/response.

**Come funziona PKCE:**

```
1) CLI genera un valore random: code_verifier
   es. "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

2) CLI calcola: code_challenge = BASE64URL(SHA256(code_verifier))
   es. "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

3) CLI manda a Keycloak solo il challenge (non il verifier):
   GET /auth?code_challenge=E9Melhoa...&code_challenge_method=S256

4) Keycloak memorizza il challenge, mostra il login

5) Utente si autentica, Keycloak manda il code alla CLI

6) CLI manda a Keycloak code + verifier (il segreto originale):
   POST /token
     code=xxx
     code_verifier=dBjftJeZ4...   ← il verifier originale

7) Keycloak verifica: SHA256(verifier) == challenge salvato?
   Se sì → rilascia i token
```

**Perché è sicuro:** anche se un attaccante intercetta il `code` dal redirect URL, non può usarlo senza il `code_verifier` che non è mai uscito dalla CLI. Il challenge (SHA256 del verifier) è pubblico ma non reversibile.

**Implementazione in aion-cli (`internal/auth/pkce.go`):**

```go
// 64 bytes random → verifier di 86 caratteri base64url
buf := make([]byte, 64)
rand.Read(buf)
verifier := base64.RawURLEncoding.EncodeToString(buf)

// SHA256 del verifier → challenge
sum := sha256.Sum256([]byte(verifier))
challenge := base64.RawURLEncoding.EncodeToString(sum[:])
```

---

### JWT — JSON Web Token

**JWT** (RFC 7519) è il formato dei token rilasciati da Keycloak. Un JWT è una stringa composta da tre parti separate da punti:

```
eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiI4Y2JkOWRlYy4uLiJ9.firma
     HEADER                    PAYLOAD               SIGNATURE
```

Ogni parte è codificata in Base64URL.

**Header** — metadati sul token:
```json
{
  "alg": "RS256",
  "typ": "JWT",
  "kid": "-WWZVRLf8yxZaO8EeliC3GXvoCmrHoYjGOE6tc1thII"
}
```

- `alg: RS256` — firma con RSA + SHA256 (asimmetrica)
- `kid` — identificativo della chiave pubblica usata per firmare

**Payload** — le claim (informazioni):
```json
{
  "sub": "8cbd9dec-73bc-4728-82f0-fbdd375431f9",
  "iss": "https://argo.datainf.cloud/realms/datainf",
  "aud": ["account"],
  "exp": 1779112326,
  "iat": 1779112026,
  "preferred_username": "stefano.denardis",
  "email": "stefano.denardis@datainf.net",
  "realm_access": {
    "roles": ["saas-admin", "grafana-superadmin"]
  }
}
```

Claim standard:
- `sub` — subject, l'ID univoco dell'utente
- `iss` — issuer, chi ha emesso il token (Keycloak)
- `exp` — expiration, timestamp Unix di scadenza
- `iat` — issued at, timestamp Unix di emissione
- `aud` — audience, a chi è destinato il token

**Signature** — firma RSA che garantisce l'integrità:
```
RSA256(
  base64url(header) + "." + base64url(payload),
  private_key_keycloak
)
```

**Come aion-api verifica il token (RS256 + JWKS):**

Keycloak espone le chiavi pubbliche su un endpoint standard:
```
GET https://argo.datainf.cloud/realms/datainf/protocol/openid-connect/certs
```

Risposta (JWKS — JSON Web Key Set):
```json
{
  "keys": [
    {
      "kid": "-WWZVRLf8yxZaO8EeliC3GXvoCmrHoYjGOE6tc1thII",
      "kty": "RSA",
      "alg": "RS256",
      "use": "sig",
      "n": "...",
      "e": "AQAB"
    }
  ]
}
```

aion-api scarica queste chiavi, le cachea, e le usa per verificare la firma di ogni token in arrivo. Non serve mai comunicare con Keycloak per ogni request — la verifica è locale e velocissima.

**Access Token vs Refresh Token:**

| | Access Token | Refresh Token |
|---|---|---|
| Durata | 5 minuti | 30 minuti — 8 ore |
| Scopo | Chiamare le API | Ottenere nuovi access token |
| Dove va | Header `Authorization: Bearer` | Solo verso Keycloak `/token` |
| Se viene rubato | Valido max 5 min | Più pericoloso |
| Algoritmo firma | RS256 | HS512 (Keycloak interno) |

---

## Flusso di autenticazione dettagliato

```
aion login
    │
    ▼
┌─────────────────────────────────────────────┐
│ 1. Genera PKCE                               │
│    verifier  = random(64 bytes) → base64url  │
│    challenge = SHA256(verifier) → base64url  │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 2. Trova porta libera su 127.0.0.1           │
│    Avvia HTTP server locale per callback     │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 3. Costruisce URL di autorizzazione          │
│    https://keycloak/realms/datainf/          │
│      protocol/openid-connect/auth            │
│      ?client_id=aion-cli                     │
│      &redirect_uri=http://127.0.0.1:PORT/cb  │
│      &response_type=code                     │
│      &scope=openid profile email             │
│      &code_challenge=<challenge>             │
│      &code_challenge_method=S256             │
│      &state=<random anti-CSRF>               │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 4. Apre il browser con quell'URL             │
│    L'utente vede la login page di Keycloak   │
│    e si autentica                            │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 5. Keycloak redirect al callback locale      │
│    http://127.0.0.1:PORT/callback            │
│      ?code=<authorization_code>              │
│      &state=<stesso state di prima>          │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 6. Server locale riceve il code              │
│    Verifica state anti-CSRF                  │
│    Mostra pagina "Login completato"          │
│    Passa il code alla CLI                    │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 7. POST /token a Keycloak                    │
│    grant_type=authorization_code             │
│    code=<authorization_code>                 │
│    redirect_uri=http://127.0.0.1:PORT/cb     │
│    code_verifier=<verifier originale>        │
│                                              │
│    Keycloak verifica:                        │
│    SHA256(verifier) == challenge ?           │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 8. Keycloak risponde con i token             │
│    {                                         │
│      "access_token": "eyJ...",               │
│      "refresh_token": "eyJ...",              │
│      "expires_in": 300,                      │
│      "refresh_expires_in": 1800              │
│    }                                         │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 9. Parse claim dal access_token              │
│    Estrae: sub, preferred_username,          │
│    email, name, realm_access.roles           │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 10. Salva in ~/.config/aion/credentials.yaml │
│     Permessi: 0600 (solo proprietario)       │
└─────────────────────────────────────────────┘
    │
    ▼
✅ Login completato!
   Utente:  stefano.denardis
   Email:   stefano.denardis@datainf.net
   Ruoli:   [saas-admin grafana-superadmin]
   Server:  https://api.datainf.cloud
```

---

## Gestione dei token

### Ciclo di vita

```
access_token  ──────────────────────────────[scade]
              │◀── 5 minuti ──▶│

refresh_token ──────────────────────────────────────────[scade]
              │◀──────────── 30 minuti ────────────────▶│
```

### Rinnovo automatico

Ogni volta che `aion-cli` deve chiamare `aion-api`, prima verifica lo stato del token:

```
token valido?
    │
    ├── SÌ → usa il token direttamente
    │
    └── NO (scade entro 30 secondi)
            │
            ├── refresh_token valido?
            │       │
            │       ├── SÌ → POST /token con refresh_token
            │       │         → nuovo access_token + refresh_token
            │       │         → aggiorna credentials.yaml
            │       │         → procedi con la chiamata
            │       │
            │       └── NO → "sessione scaduta — esegui 'aion login'"
            │
            └── (continua)
```

Il margine di 30 secondi evita race condition: se il token scade esattamente mentre la request è in volo, la chiamata fallirebbe comunque. Rinnovandolo 30 secondi prima si previene questo scenario.

### Stato del token in `aion whoami`

```
✅ valido              → token ok, scade tra N minuti
🔄 in scadenza         → verrà rinnovato alla prossima chiamata
❌ scaduto             → serve nuovo login
```

---

## Struttura del progetto

```
aion-cli/
│
├── cmd/
│   └── aion/
│       └── main.go
│           └── Punto di ingresso. Chiama commands.Execute()
│               e gestisce il codice di uscita.
│
├── commands/
│   ├── root.go
│   │   └── Comando radice "aion". Registra tutti i sottocomandi.
│   │       Usa cobra — stesso framework di kubectl.
│   │
│   ├── login.go
│   │   └── Comando "aion login".
│   │       Accetta flag: --keycloak-url, --realm,
│   │       --client-id, --server.
│   │       Avvia il flusso PKCE, salva le credenziali.
│   │
│   ├── logout.go
│   │   └── Comandi "aion logout" e "aion logout-all".
│   │       logout: cancella i token del contesto corrente.
│   │       logout-all: rimuove l'intero file credentials.yaml.
│   │
│   └── whoami.go
│       └── Comando "aion whoami".
│           Legge le credenziali salvate e mostra:
│           utente, email, ruoli, server, stato token.
│
└── internal/
    ├── auth/
    │   ├── pkce.go
    │   │   └── Genera la coppia verifier/challenge PKCE.
    │   │       verifier: 64 bytes random → base64url
    │   │       challenge: SHA256(verifier) → base64url
    │   │
    │   ├── login.go
    │   │   └── Orchestra il flusso PKCE completo:
    │   │       - trova porta libera su 127.0.0.1
    │   │       - costruisce l'URL di autorizzazione
    │   │       - avvia server HTTP locale per il callback
    │   │       - apre il browser (open/xdg-open/rundll32)
    │   │       - attende il code con timeout di 2 minuti
    │   │       - verifica il parametro state anti-CSRF
    │   │       - chiude il server locale
    │   │       - restituisce i token
    │   │
    │   └── token.go
    │       └── Funzioni per interagire con l'endpoint /token:
    │           - ExchangeCode: scambia code+verifier con token
    │           - RefreshAccessToken: rinnova con refresh_token
    │           - ParseUserInfo: estrae claim dal JWT senza
    │             verificare la firma (già verificata da Keycloak)
    │
    ├── config/
    │   └── credentials.go
    │       └── Struttura dati e I/O del file credentials.yaml.
    │           - LoadCredentials: legge il file (o crea struttura vuota)
    │           - SaveCredentials: scrive con permessi 0600
    │           - NeedsRefresh: controlla se il token sta per scadere
    │           - RefreshExpired: controlla se il refresh è scaduto
    │           - IsLoggedIn: controlla se esiste una sessione attiva
    │
    └── api/
        └── client.go
            └── Client HTTP per aion-api.
                - Aggiunge automaticamente Authorization: Bearer
                - Rinnova il token se necessario prima di ogni call
                - Salva il token rinnovato in credentials.yaml
```

---

## Configurazione Keycloak

### Client `aion-cli`

Vai su Keycloak → realm `datainf` → Clients → Create client:

| Campo | Valore |
|-------|--------|
| Client ID | `aion-cli` |
| Client authentication | **OFF** (public client) |
| Authorization | OFF |
| Standard flow | **ON** |
| Direct access grants | OFF |
| Implicit flow | OFF |
| Service accounts roles | OFF |

**Valid redirect URIs:**
```
http://localhost:*
http://127.0.0.1:*
```

**Web origins:**
```
http://localhost:*
http://127.0.0.1:*
```

### Perché public client per la CLI?

`aion-cli` è un binary distribuito — chiunque può aprirlo e leggerne il contenuto. Un `client_secret` incorporato nel binary sarebbe accessibile a chiunque, rendendo il "secret" inutile.

PKCE risolve questo problema: invece di un secret statico, usa un challenge dinamico generato per ogni login. Anche se un attaccante intercettasse il `code` nel redirect URL, non potrebbe usarlo senza il `code_verifier` che non lascia mai la CLI.

### Ruoli applicativi consigliati

```
saas-admin     → accesso completo alla piattaforma
saas-support   → accesso in sola lettura per supporto
saas-viewer    → solo visualizzazione metriche e log
```

Keycloak → realm `datainf` → Realm roles → Create role.

---

## Credenziali locali

### Posizione

```
~/.config/aion/credentials.yaml
```

### Permessi

Il file viene creato con permessi `0600` — solo il proprietario può leggerlo e scriverlo. Nessun altro utente del sistema può accedervi.

### Struttura

```yaml
version: 1
current_context: default

contexts:
  default:
    server: https://api.datainf.cloud
    realm: datainf
    client_id: aion-cli

    tokens:
      access_token: eyJhbGciOiJSUzI1NiJ9...
      refresh_token: eyJhbGciOiJIUzUxMiJ9...
      expires_at: 2026-05-18T20:05:26Z
      refresh_expires_at: 2026-05-18T20:35:26Z

    user:
      subject: 8cbd9dec-73bc-4728-82f0-fbdd375431f9
      email: stefano.denardis@datainf.net
      display_name: Stefano De Nardis
      roles:
        - saas-admin
        - grafana-superadmin
```

### Multi-contesto (futuro)

La struttura supporta già più contesti — utile per gestire ambienti diversi:

```yaml
current_context: production

contexts:
  production:
    server: https://api.datainf.cloud
    realm: datainf
    # ...

  staging:
    server: https://api-staging.datainf.cloud
    realm: datainf-staging
    # ...
```

Per ora viene usato solo `default`. In futuro si potrà aggiungere `aion context use staging`.

---

## Comandi

### `aion login`

Autentica l'utente tramite browser con PKCE flow.

```bash
# Login con valori di default
aion login

# Login con configurazione custom
aion login \
  --keycloak-url https://argo.datainf.cloud \
  --realm datainf \
  --client-id aion-cli \
  --server https://api.datainf.cloud
```

**Flag:**

| Flag | Default | Descrizione |
|------|---------|-------------|
| `--keycloak-url` | `https://argo.datainf.cloud` | URL base di Keycloak |
| `--realm` | `datainf` | Nome del realm |
| `--client-id` | `aion-cli` | Client ID configurato in Keycloak |
| `--server` | `https://api.datainf.cloud` | URL di aion-api |

**Output:**
```
🔐 Avvio autenticazione aion...
🌐 Apertura browser per autenticazione...

✅ Login completato!
   Utente:  stefano.denardis
   Email:   stefano.denardis@datainf.net
   Ruoli:   [saas-admin grafana-superadmin]
   Server:  https://api.datainf.cloud
```

---

### `aion whoami`

Mostra le informazioni sull'utente autenticato e lo stato del token.

```bash
aion whoami
```

**Output:**
```
👤 Utente corrente
   Username:         stefano.denardis
   Email:            stefano.denardis@datainf.net
   Nome:             Stefano De Nardis
   Ruoli:            saas-admin, grafana-superadmin

🔗 Connessione
   Server:           https://api.datainf.cloud
   Realm:            datainf
   Client ID:        aion-cli

🔑 Token
   Stato:            ✅ valido
   Scade tra:        4m32s
   Refresh tra:      29m32s
   Contesto:         default
```

---

### `aion logout`

Cancella i token del contesto corrente. Le informazioni di configurazione (server, realm, client_id) vengono mantenute per il prossimo login.

```bash
aion logout
```

**Output:**
```
👋 Logout completato per stefano.denardis.
```

---

### `aion logout-all`

Rimuove completamente il file `~/.config/aion/credentials.yaml`.

```bash
aion logout-all
```

**Output:**
```
🗑️  File credenziali rimosso.
```

---

## Sicurezza

### Cosa viene salvato localmente

```
✅ access_token   → JWT firmato da Keycloak, scade in 5 minuti
✅ refresh_token  → opaque token, scade in 30 minuti
✅ info utente    → username, email, ruoli (dal JWT, non password)

❌ password       → mai salvata
❌ client_secret  → non esiste (public client + PKCE)
```

### Protezioni implementate

**PKCE (Proof Key for Code Exchange)**
Protegge contro l'intercettazione del `code` OAuth. Anche se un processo malevolo sul computer intercettasse il redirect, non potrebbe scambiare il `code` per un token senza il `code_verifier`.

**State anti-CSRF**
Ogni flusso di login genera un `state` random. Il callback verifica che lo `state` ricevuto da Keycloak corrisponda a quello inviato. Previene attacchi Cross-Site Request Forgery.

**Server callback su 127.0.0.1**
Il server locale che riceve il callback da Keycloak è in ascolto su `127.0.0.1` (loopback), non su `0.0.0.0`. Non è raggiungibile dalla rete locale o da internet.

**Permessi file 0600**
Il file delle credenziali è leggibile solo dal proprietario. Su sistemi Unix, nessun altro utente (nemmeno root con restrizioni SELinux/AppArmor) può leggerlo senza privilegi espliciti.

**Token a breve durata**
L'access token dura 5 minuti. Anche se venisse intercettato (es. in un log), è inutilizzabile dopo pochi minuti.

**Nessun secret nel codice**
`aion-cli` è un public client — non contiene nessun `client_secret`. Decompilare il binary non rivela nessun segreto.

### Cosa fare se le credenziali vengono compromesse

```bash
# 1. Logout immediato dalla CLI
aion logout-all

# 2. In Keycloak — invalida tutte le sessioni dell'utente
#    Keycloak → Users → stefano.denardis → Sessions → Logout all

# 3. Se necessario, cambia la password in Keycloak
```

---

## Setup e build

### Prerequisiti

- Go 1.22 o superiore
- Client `aion-cli` configurato in Keycloak (vedi sezione dedicata)

### Build

```bash
# Clona il repository
git clone https://github.com/klonet77/aion-cli
cd aion-cli

# Installa le dipendenze
go mod tidy

# Build
go build -o aion ./cmd/aion/

# (opzionale) installa nel PATH
cp aion /usr/local/bin/
```

### Build cross-platform

```bash
# macOS ARM (M1/M2/M3/M4)
GOOS=darwin GOARCH=arm64 go build -o aion-darwin-arm64 ./cmd/aion/

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o aion-darwin-amd64 ./cmd/aion/

# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o aion-linux-amd64 ./cmd/aion/

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o aion-windows-amd64.exe ./cmd/aion/
```

### Test rapido

```bash
# Verifica che il binary funzioni
./aion --help

# Login
./aion login

# Verifica stato
./aion whoami

# Logout
./aion logout
```

---

## Riferimenti

### RFC e standard

| Standard | Titolo |
|----------|--------|
| RFC 6749 | The OAuth 2.0 Authorization Framework |
| RFC 6750 | The OAuth 2.0 Authorization Framework: Bearer Token Usage |
| RFC 7519 | JSON Web Token (JWT) |
| RFC 7517 | JSON Web Key (JWK) |
| RFC 7518 | JSON Web Algorithms (JWA) |
| RFC 7636 | Proof Key for Code Exchange (PKCE) |
| RFC 8414 | OAuth 2.0 Authorization Server Metadata |
| OpenID Connect Core 1.0 | Strato di identità su OAuth 2.0 |

### Keycloak

- [Keycloak Documentation](https://www.keycloak.org/documentation)
- [Keycloak OIDC Endpoints](https://www.keycloak.org/docs/latest/securing_apps/#endpoints)
- [Keycloak Admin REST API](https://www.keycloak.org/docs-api/latest/rest-api/)

### Librerie Go usate

- [cobra](https://github.com/spf13/cobra) — CLI framework
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — JWT parsing
- [go-yaml](https://github.com/go-yaml/yaml) — YAML serialization

### Progetti di riferimento per lo stile CLI

- [kubectl](https://github.com/kubernetes/kubectl) — CLI per Kubernetes
- [gh](https://github.com/cli/cli) — GitHub CLI
- [flyctl](https://github.com/superfly/flyctl) — Fly.io CLI
