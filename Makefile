SHELL := /usr/bin/env bash

CURRENT_DIR := $(shell pwd)

.PHONY: cleanup run_local build

run_local: build
	@rm -f node1_logs.log node2_logs.log node3_logs.log node4_logs.log node5_logs.log
	@ARGS="$(filter-out $@,$(MAKECMDGOALS))"; \
	if [ "$$ARGS" != "" ]; then \
		if [ "$$ARGS" = "filter" ] || [ "$$ARGS" = "logs" ]; then \
			../metal-network-runner/bin/metal-network-runner server \
				--log-level INFO \
				--port=":8080" \
				--grpc-gateway-port=":8081" \
			| sed \
				-e 's/2n1K5nnjF526t1hf2XX497QDe6N4eHxN3y1gstuFUJjPC9s14j/BTCVM/g' \
				-e 's/7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg/-1-/g' \
				-e 's/MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ/-2-/g' \
				-e 's/NFBbbJ4qCmNaCzeW7sxErhvWqvEQMnYcN/-3-/g' \
				-e 's/GWPcbFJZFfZreETSoWjPimr846mXEKCtu/-4-/g' \
				-e 's/P7oB2McjBGgW2NXXWVYjV8JEDFoW9xDE5/-5-/g' \
			| tee \
				>(grep '\[node1\]' > node1_logs.log) \
				>(grep '\[node2\]' > node2_logs.log) \
				>(grep '\[node3\]' > node3_logs.log) \
				>(grep '\[node4\]' > node4_logs.log) \
				>(grep '\[node5\]' > node5_logs.log) & \
		else \
			echo "Error: Invalid argument '$$ARGS'. Valid options: 'filter' or 'logs'"; \
			exit 1; \
		fi; \
	else \
		../metal-network-runner/bin/metal-network-runner server \
			--log-level info \
			--port=":8080" \
			--grpc-gateway-port=":8081" & \
	fi
	@echo $$! > metal_network_runner.pid
	@sleep 2
	@echo -n "" > /tmp/.genesis
	@PWD=$(CURRENT_DIR) envsubst < network-config.json | curl --location 'localhost:8081/v1/control/start' \
		--header 'Content-Type: application/json' \
		--data @-

cleanup:
	@pkill -F metal_network_runner.pid 2>/dev/null || true
	@rm -f metal_network_runner.pid
	@killall -9 metal-network-runner 2>/dev/null || true
	@killall -9 metalgo 2>/dev/null || true
	@killall -9 kMtihm7W3KssmcJb9mzwZfC6gkiPrJhWaa5KMLHdEB9R8Q4pp 2>/dev/null || true
	@rm -f ${CURRENT_DIR}/../metalgo/build/plugins/kMtihm7W3KssmcJb9mzwZfC6gkiPrJhWaa5KMLHdEB9R8Q4pp

build:
	./scripts/build.sh ${CURRENT_DIR}/../metalgo/build/plugins/kMtihm7W3KssmcJb9mzwZfC6gkiPrJhWaa5KMLHdEB9R8Q4pp
