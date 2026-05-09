Implement V1 from synthesis modulo the three design-review
findings. Ship: chat lifecycle (POST /chat/new, GET
/chat/<id>, POST .../send, GET .../events SSE, POST
.../stop), Markdown rendering on artifact pages, minimum
file-view endpoint if synthesis includes it. New module
striatum.web.chat_provider with two flavor clients
(AnthropicMessages, OpenAIChat). striatum.web.markdown for
markdown-it-py + sanitizer. Tests, docs, version bump to
v1.12.0.
