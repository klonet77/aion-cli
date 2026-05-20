# aion-cli

CLI ufficiale per aion-api.

## Setup

```bash
cd aion-cli
go mod tidy
go build -o aion ./cmd/aion/
```

## Comandi

```bash
# Login tramite browser (PKCE)
./aion login

# Login con server/realm custom
./aion login \
  --keycloak-url https://argo.datainf.cloud \
  --realm datainf \
  --client-id aion-cli \
  --server https://api.datainf.cloud



# Mostra utente corrente
./aion whoami

# Logout (cancella token corrente)
./aion logout

# Logout completo (cancella tutto il file)
./aion logout-all
```

## Configurazione Keycloak

Prima di usare la CLI, configura il client `aion-cli` in Keycloak:

```
Client ID:              aion-cli
Client authentication:  OFF  (public client, PKCE non richiede secret)
Standard flow:          ON
Direct access grants:   OFF
Valid redirect URIs:    http://localhost:*
                        http://127.0.0.1:*
Web origins:            http://localhost:*
```

## Credenziali

Le credenziali vengono salvate in:

```
~/.config/aion/credentials.yaml
```

Il file ha permessi `0600` — solo il proprietario può leggerlo.

## Struttura progetto

```
aion-cli/
├── cmd/aion/main.go              # entry point
├── commands/
│   ├── root.go                   # comando root + registrazione
│   ├── login.go                  # aion login
│   ├── logout.go                 # aion logout / logout-all
│   └── whoami.go                 # aion whoami
├── internal/
│   ├── auth/
│   │   ├── pkce.go               # generazione PKCE verifier/challenge
│   │   ├── login.go              # flusso browser + callback locale
│   │   └── token.go              # scambio code, refresh, parse claims
│   ├── config/
│   │   └── credentials.go        # struttura e read/write credentials.yaml
│   └── api/
│       └── client.go             # client HTTP con refresh automatico
└── go.mod
```

## Flusso di autenticazione

```
aion login
    ↓
genera PKCE verifier + challenge
    ↓
avvia server locale su porta casuale (127.0.0.1:PORT)
    ↓
apre browser → Keycloak login page
    ↓
utente si autentica
    ↓
Keycloak redirect → http://127.0.0.1:PORT/callback?code=xxx
    ↓
CLI scambia code + verifier con access_token + refresh_token
    ↓
salva in ~/.config/aion/credentials.yaml
```

## Rinnovo automatico token

Ogni volta che la CLI chiama aion-api, verifica se l'access token
sta per scadere (entro 30 secondi). Se sì, usa il refresh token
per ottenerne uno nuovo silenziosamente.

Se anche il refresh token è scaduto, mostra il messaggio:

```
sessione scaduta — esegui 'aion login'
```
