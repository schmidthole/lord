.PHONY: add-user clean

REGISTRY_HOST = registry.onyxalgo.com
HTPASSWD_FILE = htpasswd
CLIENT_CONFIGS_DIR = client-configs

add-user:
	@mkdir -p $(CLIENT_CONFIGS_DIR)
	@read -p "username: " USERNAME; \
	PASSWORD=$$(openssl rand -base64 32); \
	echo "generated password: $$PASSWORD"; \
	if [ ! -f $(HTPASSWD_FILE) ]; then \
		htpasswd -cb $(HTPASSWD_FILE) $$USERNAME $$PASSWORD; \
	else \
		htpasswd -b $(HTPASSWD_FILE) $$USERNAME $$PASSWORD; \
	fi; \
	AUTH_STRING=$$(echo -n "$$USERNAME:$$PASSWORD" | base64); \
	echo "{" > $(CLIENT_CONFIGS_DIR)/$$USERNAME-config.json; \
	echo "  \"auths\": {" >> $(CLIENT_CONFIGS_DIR)/$$USERNAME-config.json; \
	echo "    \"$$REGISTRY_HOST\": {" >> $(CLIENT_CONFIGS_DIR)/$$USERNAME-config.json; \
	echo "      \"auth\": \"$$AUTH_STRING\"" >> $(CLIENT_CONFIGS_DIR)/$$USERNAME-config.json; \
	echo "    }" >> $(CLIENT_CONFIGS_DIR)/$$USERNAME-config.json; \
	echo "  }" >> $(CLIENT_CONFIGS_DIR)/$$USERNAME-config.json; \
	echo "}" >> $(CLIENT_CONFIGS_DIR)/$$USERNAME-config.json; \
	echo "docker config generated: $(CLIENT_CONFIGS_DIR)/$$USERNAME-config.json"

clean:
	rm -f $(HTPASSWD_FILE)
	rm -rf $(CLIENT_CONFIGS_DIR)