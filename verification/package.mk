.PHONY: docs interoperability soak

docs:
	./scripts/check-docs.sh

interoperability:
	bash ./scripts/check-interoperability.sh candidate

soak:
	bash ./scripts/check-soak.sh
