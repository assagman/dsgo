# Snowflake Research Report Generator

This example demonstrates a complete multi-agent system that researches a topic (default: Snowflake Data Cloud) and generates a comprehensive markdown research report with structured curriculum content.

## 🏗️ Architecture

The system uses a pipeline of specialized agents:

1. **ChatAgent**: Parses your learning request and determines objectives
2. **SupervisorAgent**: Plans research strategy and breaks down topics
3. **WebResearchAgent**: Executes parallel web research using MCP tools (Exa/Tavily)
4. **CombinerAgent**: Synthesizes findings into a unified knowledge base
5. **CurriculumAgent**: Transforms knowledge into structured learning modules, quizzes, and exercises
6. **ReportGenerator**: Renders the final markdown research report

## 🚀 Usage

### Prerequisites

- Go 1.21+
- API Key for LLM (OpenRouter or OpenAI)
- (Optional) API Keys for Exa and Jina (for better research)

### Environment Setup

```bash
# Required
export OPENROUTER_API_KEY="sk-or-..." 
# OR
export OPENAI_API_KEY="sk-..."

# Optional (highly recommended for research quality)
export EXA_API_KEY="..."
export TAVILY_API_KEY="..."

# Optional Configuration
export TRAINER_MODEL="openrouter/google/gemini-2.5-flash-lite-preview-09-2025"
export TRAINER_MAX_WORKERS="6"
export TRAINER_TOPIC="Snowflake Data Cloud"
```

### Running the Report Generator

Navigate to the project directory:

```bash
cd examples/snowflake_trainer
```

Run with a specific learning request:

```bash
go run . "Teach me Snowflake performance tuning for a data engineer interview"
```

Or just run defaults:

```bash
go run .
```

### Output

Generates **both** `research_report_*.md` (Markdown) and `research_report_*.html` (interactive HTML with collapsible sections, responsive design, print styles) in the `output/` directory.

**Markdown**: View in VS Code, Typora, GitHub, etc.  
**HTML**: Open in browser for best interactive/print experience!

## 📄 Report Structure

The generated markdown report includes:

- **Executive Summary**: Key takeaways and overview
- **Table of Contents**: Easy navigation with links
- **Learning Modules**: Detailed content with objectives, concepts, and examples
- **Quiz Assessment**: Comprehensive questions with answer keys
- **Practical Exercises**: Hands-on labs with solutions
- **Challenges**: Real-world scenarios to test skills
- **Glossary**: Key terms and definitions
- **Additional Resources**: Curated links and references
- **Research Sources**: Citations and references

## 🧩 Key DSGo Patterns Used

- **Typed Signatures**: Type-safe I/O for all agents
- **Chain of Thought**: For planning and curriculum design
- **ReAct Pattern**: For web research
- **Parallel Execution**: For concurrent research on sub-topics
- **Multi-Chain Comparison**: For synthesizing diverse findings
- **Structured Output**: JSON generation for quizzes and exercises

## 🛠️ Customization

You can adapt this report generator for any topic by setting the `TRAINER_TOPIC` environment variable or just asking a different question in the CLI argument.

Example:
```bash
export TRAINER_TOPIC="Kubernetes Security"
go run . "Create a comprehensive report on Kubernetes security best practices"
```

## 📊 Example Output

**Markdown**: Professional formatting with tables, code blocks, TOC.  
**HTML**: Adds responsive design, collapsible quizzes/exercises, sticky navigation, badges, print styles (no JS/external deps).
