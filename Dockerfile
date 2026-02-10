# Estágio de Build
FROM golang:1.22-alpine AS builder

# Instala dependências do sistema necessárias para CGO (SQLite)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copia arquivos de dependência primeiro para aproveitar cache
COPY go.mod go.sum ./
RUN go mod download

# Copia o código fonte
COPY . .

# Compila o binário
# CGO_ENABLED=1 é necessário para go-sqlite3
# -ldflags="-s -w" remove símbolos de debug para reduzir tamanho
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o woll-find main.go

# Estágio de Runtime
FROM alpine:latest

WORKDIR /app

# Instala dependências mínimas (se necessário, ex: ca-certificates)
RUN apk add --no-cache ca-certificates tzdata

# Copia o binário do estágio de build
COPY --from=builder /app/woll-find .

# Copia templates e arquivos estáticos
COPY --from=builder /app/views ./views
COPY --from=builder /app/public ./public

# Cria diretórios para volumes
RUN mkdir -p /app/database /app/public/uploads

# Expõe a porta da aplicação
EXPOSE 3000

# Comando de inicialização
CMD ["./woll-find"]
