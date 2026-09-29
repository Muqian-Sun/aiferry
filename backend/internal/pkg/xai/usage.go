package xai

// IncludeIndependentReasoningTokens adds reasoning tokens to billed output
// only when total_tokens proves they are not already folded into output.
//
// Official xAI Chat Completions example: prompt=32, completion=9,
// reasoning=94, total=135. Responses example: input=32, output=9,
// reasoning=110, total=151. OpenAI's canonical completion_tokens already
// includes reasoning and total equals input+output.
//
// Reasoning larger than the visible output can never be folded into it (the
// OpenAI inclusive shape guarantees reasoning <= completion), so it is always
// added — even when total_tokens is absent or also leaves it out. Some relays
// fronting xAI models report total = input + output on streaming responses
// (seen 2026-09-29: output=1, reasoning=59, total=input+1), which the total
// check alone would under-bill.
func IncludeIndependentReasoningTokens(input, output, total, reasoning int64) int64 {
	if input < 0 || output < 0 || reasoning <= 0 {
		return output
	}
	if reasoning > output {
		return output + reasoning
	}
	if total <= 0 {
		return output
	}
	if total == input+output {
		return output
	}
	gap := total - input - output
	if gap <= 0 {
		return output
	}
	if reasoning < gap {
		gap = reasoning
	}
	return output + gap
}
