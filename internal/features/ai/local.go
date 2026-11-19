package ai

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxTitleLength adalah panjang maksimal judul artikel (karakter).
const MaxTitleLength = 120

const (
	// GeneratorOpenAI menandai artikel yang ditulis oleh OpenAI Chat Completions.
	GeneratorOpenAI = "openai"
	// GeneratorLocal menandai artikel yang disusun penulis lokal (tanpa layanan LLM).
	GeneratorLocal = "local"
)

// LocalTitle membuat judul dari topik: huruf pertama kapital dan maksimal MaxTitleLength karakter.
func LocalTitle(topic string) string {
	title := truncate(strings.TrimSpace(topic), MaxTitleLength-3)
	first, size := utf8.DecodeRuneInString(title)
	return string(unicode.ToUpper(first)) + title[size:]
}

// BuildLocalArticle menyusun draf artikel Markdown tanpa layanan LLM, berdasarkan topik,
// kategori, gaya bahasa dan hasil penelusuran mesin pencari (dikutip sebagai [1], [2], ...).
// Dipakai sebagai cadangan ketika OPENAI_KEY / OPENAI_MODEL belum diatur atau layanan AI gagal.
func BuildLocalArticle(topic, category, tone string, sources []SearchResult) *GeneratedArticle {
	var b strings.Builder

	b.WriteString("## Pendahuluan\n\n")
	fmt.Fprintf(&b, "**%s** merupakan salah satu topik dalam kategori %s yang semakin banyak dibicarakan. ", topic, category)
	b.WriteString("Perkembangan informasi yang cepat membuat kita perlu memahami topik ini secara utuh, mulai dari konsep dasar, ")
	b.WriteString("manfaat dan tantangannya, hingga langkah praktis yang dapat langsung diterapkan.\n\n")
	if tone != "" {
		fmt.Fprintf(&b, "Artikel ini disajikan dengan gaya bahasa %s agar mudah diikuti oleh pembaca.\n\n", tone)
	}

	b.WriteString("## Poin-Poin Penting\n\n")
	if len(sources) == 0 {
		fmt.Fprintf(&b, "- **Konsep dasar**: pahami definisi dan ruang lingkup %s sebelum mendalami detailnya.\n", topic)
		b.WriteString("- **Manfaat**: kenali dampak positif yang dapat dirasakan oleh individu maupun organisasi.\n")
		b.WriteString("- **Tantangan**: perhatikan kendala, risiko, dan hal yang perlu diantisipasi sejak awal.\n")
		b.WriteString("- **Tren terkini**: ikuti perkembangan terbaru agar pemahaman tetap relevan.\n\n")
	} else {
		b.WriteString("Berikut rangkuman fakta terkini dari hasil penelusuran mesin pencari:\n\n")
		for i, s := range sources {
			snippet := s.Snippet
			if snippet == "" {
				snippet = "Lihat sumber untuk informasi selengkapnya."
			}
			fmt.Fprintf(&b, "- **%s**: %s [%d]\n", s.Title, snippet, i+1)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Pembahasan\n\n")
	fmt.Fprintf(&b, "Dalam praktiknya, %s tidak dapat dilepaskan dari konteks kebutuhan pembaca dan lingkungan sekitarnya. ", topic)
	b.WriteString("Pendekatan yang baik dimulai dari memahami masalah yang ingin diselesaikan, lalu memilih cara yang paling sesuai ")
	b.WriteString("dengan sumber daya yang tersedia. Evaluasi secara berkala juga penting agar hasil yang diperoleh terus meningkat.\n\n")

	b.WriteString("## Langkah Praktis\n\n")
	b.WriteString("1. Pelajari konsep dasarnya dari sumber yang tepercaya.\n")
	b.WriteString("2. Mulai dari langkah kecil dan ukur hasilnya.\n")
	b.WriteString("3. Diskusikan pengalaman Anda dengan komunitas atau rekan sejawat.\n")
	b.WriteString("4. Terus perbarui pengetahuan mengikuti perkembangan terbaru.\n\n")

	b.WriteString("## Kesimpulan\n\n")
	fmt.Fprintf(&b, "%s menawarkan banyak peluang sekaligus tantangan. Dengan pemahaman yang tepat dan langkah yang terukur, ", LocalTitle(topic))
	b.WriteString("topik ini dapat memberikan manfaat nyata bagi pembaca. Silakan sunting draf ini sesuai kebutuhan sebelum dipublikasikan.\n")

	return &GeneratedArticle{
		Title:    LocalTitle(topic),
		Category: category,
		Excerpt:  fmt.Sprintf("Ulasan ringkas tentang %s: konsep dasar, poin penting, dan langkah praktis yang bisa langsung diterapkan.", topic),
		Content:  b.String(),
	}
}
