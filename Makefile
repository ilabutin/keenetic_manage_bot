BINARY       = keenetic-bot
LDFLAGS      = -ldflags="-s -w"
ROUTER_HOST ?= 172.16.0.1
ROUTER_USER ?= root

.PHONY: build build-router deploy run tidy

# Сборка под текущую платформу (для разработки)
build:
	go build $(LDFLAGS) -o $(BINARY) .

# Keenetic ILN-Main-Router (Linux 4.9, mipsel-3.4, little-endian, softfloat)
build-router:
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build $(LDFLAGS) -o $(BINARY)-mipsle .

# Собрать и залить бинарник + init-скрипт на роутер
deploy: build-router
	ssh $(ROUTER_USER)@$(ROUTER_HOST) '/opt/etc/init.d/S99keenetic-bot stop 2>/dev/null; true'
	base64 -i $(BINARY)-mipsle | ssh $(ROUTER_USER)@$(ROUTER_HOST) \
		'base64 -d > /opt/sbin/keenetic-bot && chmod +x /opt/sbin/keenetic-bot'
	cat init.d/S99keenetic-bot | ssh $(ROUTER_USER)@$(ROUTER_HOST) \
		'cat > /opt/etc/init.d/S99keenetic-bot && chmod +x /opt/etc/init.d/S99keenetic-bot'
	ssh $(ROUTER_USER)@$(ROUTER_HOST) '/opt/etc/init.d/S99keenetic-bot start'
	@echo "Done."

run:
	go run . -config config.yaml

tidy:
	go mod tidy
