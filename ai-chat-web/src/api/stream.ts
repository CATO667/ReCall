export interface ChatStreamPayload {
  prompt: string
  options?: { conversationId?: string; parentMessageId?: string }
  systemMessage?: string
}

export async function readChatProcessStream(
  payload: ChatStreamPayload,
  onMessage: (data: Chat.ConversationResponse & { statusHint?: string }) => void,
  signal?: AbortSignal,
): Promise<void> {
  const baseURL = import.meta.env.VITE_GLOB_API_URL || '/api'
  const url = `${baseURL.replace(/\/$/, '')}/chat-process`

  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      prompt: payload.prompt,
      options: payload.options,
      systemMessage: payload.systemMessage,
    }),
    signal,
    credentials: 'same-origin',
  })

  if (!response.ok)
    throw new Error(`chat-process failed: ${response.status}`)

  if (!response.body)
    throw new Error('chat-process response body is empty')

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done)
      break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''

    for (const line of lines) {
      const trimmed = line.trim()
      if (!trimmed)
        continue
      onMessage(JSON.parse(trimmed))
    }
  }

  const tail = buffer.trim()
  if (tail)
    onMessage(JSON.parse(tail))
}
