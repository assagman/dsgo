# DSGo Roadmap

*Note: The DSGo implementation is organized in internal packages (`internal/core/`, `internal/module/`, `internal/logging/`, `internal/typed/`, etc.) for better maintainability, while maintaining the same public API through re-exports in the main package.*

---

## ✅ COMPLETED PHASES

### Phase 1: Core Foundation
- ✅ LM interface with Generate/Stream
- ✅ Signature system with field types
- ✅ Module interface
- ✅ 7 everyday modules: Predict, ChainOfThought, ReAct, ProgramOfThought, BestOfN, Refine, Program
- ✅ Tool/ToolCall support
- ✅ History & Prediction wrappers

### Phase 2: Adapters
- ✅ JSONAdapter with automatic repair
- ✅ ChatAdapter with robust parsing
- ✅ TwoStepAdapter for reasoning
- ✅ FallbackAdapter (DSGo exclusive)

### Phase 3: Configuration & Observability
- ✅ Configure() with functional options
- ✅ Settings & environment variables
- ✅ Rich HistoryEntry (usage, cost, latency, metadata)
- ✅ Collectors (Memory, JSONL, Composite)
- ✅ LMWrapper auto-instrumentation
- ✅ OpenAI & OpenRouter providers

### Phase 4: Observability Parity
- ✅ Provider metadata persistence
- ✅ Streaming observability
- ✅ Cache improvements (keys, stats, deep copy)
- ✅ Provider naming standardization

### Phase 5: Typed Signatures
- ✅ typed.Func[I, O] with generics
- ✅ Struct tag parsing
- ✅ Type-safe few-shot examples

---

## 📋 PLANNED PHASES

### Phase 6: Advanced Modules
- ✅ **6.1: Parallel Module** - Worker pools, error aggregation, metrics
- ✅ **6.2: MultiChainComparison** - Generate N outputs, LM-based synthesis
- ⏳ **6.3: KNN** - Vector similarity for few-shot (depends on Phase 7)
- ⏳ **6.4: CodeAct** - Code interpreter + tools (safety-gated execution)

### Phase 7: Embeddings & Retrieval
- ⏳ **7.1: Embedder Interface** - Embed(ctx, texts) method
- ⏳ **7.2: Provider Support** - OpenAI embeddings (text-embedding-3-small/large)
- ⏳ **7.3: Vector Operations** - Cosine similarity, L2 distance
- ⏳ **7.4: Retrieval Integration** - RAG workflows, FAISS integration
- ⏳ **7.5: Storage & Persistence** - Save/load embeddings

### Phase 8: Multimodal Support
- ✅ **8.1: Image type** exists (partial)
- [ ] **8.1: Enhanced Image Support** - Base64 encoding, vision models
- [ ] **8.2: Audio Primitive** - Whisper integration, format support
- [ ] **8.3: Document Support** - PDF extraction, citations
- [ ] **8.4: Adapter Updates** - Multimodal serialization

### Phase 9: Additional Providers
- ✅ **OpenAI** - Complete
- ✅ **OpenRouter** - Complete
- [ ] **9.1: Groq** - Fast inference models (HIGH priority)
- [ ] **9.2: Cerebras** - High-speed inference (MEDIUM priority)

### Phase 10: Advanced Infrastructure
- ⏳ **10.1: Enhanced Caching** - TTL expiry, disk cache, auto-wiring (HIGH)
- ⏳ **10.2: Enhanced Retry** - Retry-After header, configurable params (MEDIUM)
- ⏳ **10.3: Streaming Enhancements** - CoT, ReAct, PoT, Refine streaming (MEDIUM)
- ⏳ **10.4: Async Support** - aforward() equivalents with goroutines (LOW)
- ⏳ **10.5: Callback System** - BaseCallback interface, hooks (LOW)
- ⏳ **10.6: Utilities** - Save/load programs, serialization (LOW)
- ⏳ **10.7: Parallel Enhancements** - Straggler detection, progress bar (MEDIUM)

---

## 🎯 NEXT PRIORITIES

1. Phase 10
2. Phase 6
