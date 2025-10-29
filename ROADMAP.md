# DSGo Implementation Roadmap

**Goal**: Complete Go port of DSPy framework based on [official Python API](https://dspy.ai/api/)

**Status**: Phase 1 Complete ✅ | Ready for Phase 2

---

## ✅ Completed Components

### Core Architecture
- [x] `LM` interface (language models)
- [x] `Signature` (input/output field definitions)
- [x] `Module` interface
- [x] `Message`, `GenerateOptions`, `GenerateResult`
- [x] Field types (string, int, float, bool, json, class, image, datetime)

### Modules
- [x] `Predict` - Basic prediction
- [x] `ChainOfThought` - Reasoning module
- [x] `ReAct` - Tool-using agent
- [x] `ProgramOfThought` - Code generation/execution
- [x] `BestOfN` - Generate N, pick best
- [x] `Refine` - Iterative refinement
- [x] `Program` - Module composition/pipeline

### Primitives
- [x] `Tool` - Tool/function calling
- [x] `ToolCall` - Tool invocation
- [x] `History` - Conversation history management
- [x] `Prediction` - Rich prediction wrapper with metadata
- [x] `Example` / `ExampleSet` - Few-shot learning support

### LM Providers
- [x] OpenAI provider
- [x] OpenRouter provider
- [x] Provider auto-detection helper

### Examples
- [x] 10 working examples
  - sentiment, react_agent, content_generator, customer_support
  - math_solver, composition, data_analyst, code_reviewer
  - research_assistant, **fewshot_conversation** (NEW)

---

## 🚧 Implementation Plan

### **Phase 1: Core Primitives** (Priority: HIGH) ✅ COMPLETE

1. **History** - Conversation history management
   - [x] `History` type for managing conversation context
   - [x] Methods: `Add()`, `Get()`, `Clear()`, `GetLast(n)`
   - [x] Size limiting and cloning support
   - [x] Comprehensive tests
   - [x] Example demonstrating multi-turn conversations

2. **Prediction** - Structured prediction wrapper
   - [x] `Prediction` type wrapping outputs + metadata
   - [x] Include rationale, completions, usage stats
   - [x] Type-safe getter methods
   - [x] Comprehensive tests
   - [x] Integration examples

3. **Example** - Few-shot learning support
   - [x] `Example` type for input/output pairs
   - [x] `ExampleSet` for managing collections
   - [x] Integration with signatures for few-shot prompting
   - [x] Format examples for prompts
   - [x] Comprehensive tests
   - [x] Example demonstrating few-shot learning

### **Phase 2: Adapters** (Priority: MEDIUM)

4. **Base Adapter Interface**
   - [ ] `Adapter` interface for format conversion
   - [ ] Signature to prompt conversion abstraction
   - [ ] LM response parsing abstraction

5. **ChatAdapter**
   - [ ] Convert signatures to chat format
   - [ ] Handle multi-turn conversations
   - [ ] System prompt construction

6. **JSONAdapter**
   - [ ] Structured JSON response parsing
   - [ ] Schema generation from signatures
   - [ ] Validation and error recovery

### **Phase 3: Advanced Modules** (Priority: MEDIUM-LOW)

7. **Parallel**
   - [ ] Execute multiple modules in parallel
   - [ ] Result aggregation strategies
   - [ ] Error handling for parallel execution
   - [ ] Example with parallel processing

8. **MultiChainComparison**
   - [ ] Generate multiple reasoning chains
   - [ ] Compare and select best chain
   - [ ] Voting/consensus mechanisms

9. **CodeAct**
   - [ ] Advanced code action agent
   - [ ] Code execution environment
   - [ ] Safety sandboxing

### **Phase 4: Utilities** (Priority: MEDIUM-LOW)

10. **Caching System**
    - [ ] `configure_cache()` for LM response caching
    - [ ] Cache key generation
    - [ ] TTL and invalidation
    - [ ] In-memory and persistent options

11. **Streaming Support**
    - [ ] `StreamListener` interface
    - [ ] Streaming for compatible LMs
    - [ ] Token-by-token callbacks
    - [ ] Progress indicators

12. **Logging Utilities**
    - [ ] `enable_logging()` / `disable_logging()`
    - [ ] Structured logging for debugging
    - [ ] LM call tracing
    - [ ] Performance metrics

13. **Save/Load Functionality**
    - [ ] `save()` / `load()` for programs
    - [ ] Serialization of modules
    - [ ] Example persistence
    - [ ] Configuration export/import

### **Phase 5: Embeddings** (Priority: MEDIUM)

14. **Embedder Interface**
    - [ ] `Embedder` interface for embedding models
    - [ ] Batch embedding support
    - [ ] Dimension and normalization options

15. **OpenAI Embeddings**
    - [ ] `text-embedding-3-small` / `text-embedding-3-large`
    - [ ] OpenRouter embeddings support
    - [ ] Usage tracking

### **Phase 6: Multimodal** (Priority: LOW)

16. **Audio Primitive**
    - [ ] `Audio` type for audio inputs
    - [ ] Format support (mp3, wav, etc.)
    - [ ] Integration with compatible LMs

17. **Image Primitive**
    - [ ] Full `Image` implementation (type exists)
    - [ ] Base64 encoding
    - [ ] URL support
    - [ ] Vision model integration

---

## 📊 Progress Tracking

### Component Coverage
- **Modules**: 7/10 (70%)
- **Primitives**: 5/7 (71%) ⬆️
- **Adapters**: 0/3 (0%)
- **Utils**: 0/4 (0%)
- **Models**: 1/2 (50%)

### Overall Completion
- **Core Features**: ~70% ⬆️
- **Advanced Features**: ~40%
- **Complete DSPy Parity**: ~55% ⬆️

---

## 🎯 Next Immediate Steps

1. ✅ ~~Add OpenRouter support~~
2. ✅ ~~Update examples to support both providers~~
3. ✅ ~~Implement History primitive~~
4. ✅ ~~Implement Prediction primitive~~
5. ✅ ~~Implement Example primitive for few-shot~~
6. **Begin Phase 2: Adapters**

---

## 📝 Notes

- **Excluded**: Optimizers and Evaluation (as requested)
- **Testing**: Each phase should include comprehensive tests
- **Examples**: Add new examples as features are implemented
- **Documentation**: Update README and docs with each phase

---

## 🔄 Updates

- **2025-10-28**: 
  - Initial roadmap created
  - ✅ OpenRouter support added
  - ✅ All examples updated to support both OpenAI and OpenRouter
  - ✅ **Phase 1 Complete**: History, Prediction, Example primitives implemented
  - ✅ Comprehensive tests added for all new primitives
  - ✅ New example: fewshot_conversation demonstrating all Phase 1 features
