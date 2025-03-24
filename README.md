# Corpus

A "good enough" and easy to deploy [RAG](https://en.wikipedia.org/wiki/Retrieval-augmented_generation) service.

> ⚠️ **Disclaimer**
>
> Corpus is currently under active development and should be considered a work in progress. The API is in a preliminary stage and may not be stable. Please be aware that changes, including modifications, updates, or deprecations, can occur at any time without prior notice.

## Getting started

### With Docker

```bash
docker run \
  -it --rm \
  -v corpus_data:/data \
  --net=host \
  -e CORPUS_LLM_PROVIDER_KEY="<LLM_SERVICE_API_KEY>" \
  -e CORPUS_LLM_PROVIDER_BASE_URL="<LLM_SERVICE_BASE_URL>" \
  -e CORPUS_LLM_PROVIDER_CHAT_COMPLETION_MODEL="<LLM_SERVICE_CHAT_COMPLETION_MODEL>" \
  -e CORPUS_LLM_PROVIDER_EMBEDDINGS_MODEL="<LLM_SERVICE_EMBEDDINGS_MODEL>" \
  ghcr.io/bornholm/corpus:latest
```

Then open http://localhost:3002 in your browser, default credentials are `corpus` / `corpus`.

**Examples**

With a local [ollama](https://ollama.com/) instance:

```bash
CORPUS_LLM_PROVIDER_BASE_URL=http://127.0.0.1:11434/v1/
CORPUS_LLM_PROVIDER_CHAT_COMPLETION_MODEL=qwen2.5:7b
CORPUS_LLM_PROVIDER_EMBEDDINGS_MODEL=mxbai-embed-large
```

With [Mistral](https://mistral.ai/):

```bash
CORPUS_LLM_PROVIDER_BASE_URL=https://api.mistral.ai/v1/
CORPUS_LLM_PROVIDER_CHAT_COMPLETION_MODEL=mistral-small-latest
CORPUS_LLM_PROVIDER_EMBEDDINGS_MODEL=mistral-embed
```

## License

[AGPL-3.0](LICENSE.md)
