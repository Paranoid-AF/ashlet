LLAMA_CPP_DIR := llama.cpp
MODELS_DIR := models

.PHONY: build setup clean

build: $(LLAMA_CPP_DIR)
	cd $(LLAMA_CPP_DIR) && cmake -B build -DGGML_METAL=ON && cmake --build build --config Release

setup: $(LLAMA_CPP_DIR) $(MODELS_DIR)
	./scripts/setup_model.sh

$(LLAMA_CPP_DIR):
	git clone --depth 1 https://github.com/ggerganov/llama.cpp.git $(LLAMA_CPP_DIR)

$(MODELS_DIR):
	mkdir -p $(MODELS_DIR)

clean:
	rm -rf $(LLAMA_CPP_DIR)/build
