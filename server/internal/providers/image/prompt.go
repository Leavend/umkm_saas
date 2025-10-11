package image

import (
	"fmt"
	"strings"

	"server/internal/domain/jsoncfg"
)

// DefaultNegativePrompt captures undesirable artefacts we want the model to avoid.
const DefaultNegativePrompt = "low quality, blurry, distorted, washed out, incorrect anatomy, extra limbs, text artefacts, watermark"

// BuildMarketingPrompt converts the structured prompt JSON into a natural language
// instruction tailored for text-to-image models. The prompt emphasises branding,
// photography direction, locale, and any creative constraints required by the
// business workflow.
func BuildMarketingPrompt(p jsoncfg.PromptJSON) string {
	var lines []string

	title := strings.TrimSpace(p.Title)
	if title != "" {
		lines = append(lines, fmt.Sprintf("Create a premium marketing photograph for %q.", title))
	} else {
		lines = append(lines, "Create a premium marketing photograph for the featured product.")
	}

	if product := strings.TrimSpace(p.ProductType); product != "" {
		lines = append(lines, fmt.Sprintf("Product category: %s.", product))
	}

	var stylistic []string
	if style := strings.TrimSpace(p.Style); style != "" {
		stylistic = append(stylistic, fmt.Sprintf("visual style %q", style))
	}
	if bg := strings.TrimSpace(p.Background); bg != "" {
		stylistic = append(stylistic, fmt.Sprintf("background %q", bg))
	}
	if len(stylistic) > 0 {
		lines = append(lines, "Visual direction: "+strings.Join(stylistic, ", ")+".")
	}

        if instr := strings.TrimSpace(p.Instructions); instr != "" {
                lines = append(lines, fmt.Sprintf("Creative guidance: %s.", instr))
        }

        if len(p.References) > 0 {
                refs := make([]string, 0, len(p.References))
                for _, ref := range p.References {
                        ref = strings.TrimSpace(ref)
                        if ref != "" {
                                refs = append(refs, ref)
                        }
                }
                if len(refs) > 0 {
                        lines = append(lines, "Inspiration references: "+strings.Join(refs, "; "))
                }
        }

        if !p.SourceAsset.IsZero() {
                lines = append(lines, "Use the uploaded product photo as the main subject. Preserve its shape, texture, and logo without warping.")
        }

        switch p.Workflow.Mode {
        case jsoncfg.WorkflowModeBackground:
                theme := strings.TrimSpace(p.Workflow.BackgroundTheme)
                if theme == "" {
                        theme = "an on-brand, aesthetic setting"
                }
                style := strings.TrimSpace(p.Workflow.BackgroundStyle)
                if style != "" {
                        theme = fmt.Sprintf("%s with %s style", theme, style)
                }
                lines = append(lines,
                        fmt.Sprintf("Replace only the background with %s while keeping the product lighting consistent and natural.", theme))
        case jsoncfg.WorkflowModeEnhance:
                boost := strings.TrimSpace(p.Workflow.EnhanceLevel)
                if boost == "" {
                        boost = "balanced"
                }
                lines = append(lines,
                        fmt.Sprintf("Enhance the original photo with %s colour grading, improved brightness, and crisp contrast while avoiding oversaturation.", boost))
        case jsoncfg.WorkflowModeRetouch:
                strength := strings.TrimSpace(p.Workflow.RetouchStrength)
                if strength == "" {
                        strength = "gentle"
                }
                lines = append(lines,
                        fmt.Sprintf("Retouch blemishes using a %s touch so the product remains authentic and appetising.", strength))
        default:
                // fallthrough: standard generation already covered by default statements above
        }

        if note := strings.TrimSpace(p.Workflow.Notes); note != "" {
                lines = append(lines, fmt.Sprintf("Additional workflow note: %s.", note))
        }

        if p.Watermark.Enabled {
                watermark := strings.TrimSpace(p.Watermark.Text)
                if watermark != "" {
                        position := strings.TrimSpace(p.Watermark.Position)
                        if position == "" {
				position = "bottom-right"
			}
			lines = append(lines, fmt.Sprintf("Embed the brand watermark text %q at the %s of the composition in a subtle yet readable style.", watermark, position))
		}
	}

	quality := strings.TrimSpace(p.Extras.Quality)
	if quality == "" {
		quality = jsoncfg.DefaultExtrasQuality
	}
	lines = append(lines, fmt.Sprintf("Render with %s quality lighting, sharp focus, and clean post-processing.", quality))

	locale := strings.TrimSpace(p.Extras.Locale)
	if locale == "" {
		locale = jsoncfg.DefaultExtrasLocale
	}
	lines = append(lines, fmt.Sprintf("Use %s language for any on-image typography or signage.", strings.ToUpper(locale)))

	lines = append(lines, "Ensure the scene looks appetising, well-lit, and ready for social media or menu promotion.")

	return strings.Join(lines, "\n")
}

// AspectRatioSize maps an aspect ratio string to the DashScope supported size token.
func AspectRatioSize(aspect string) string {
	switch strings.TrimSpace(aspect) {
	case "16:9":
		return "1664*928"
	case "4:3":
		return "1472*1104"
	case "3:4":
		return "1140*1472"
	case "9:16":
		return "928*1664"
	default:
		return "1328*1328"
	}
}
