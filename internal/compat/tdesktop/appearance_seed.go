package tdesktop

import (
	"sync"

	"telesrv/internal/seed/appearance"

	"github.com/gotd/td/tg"
)

const appearanceSeedDCID = 2
const maxPeerColorBoostLevel = 100

var peerColorOptionsCache = struct {
	regularOnce sync.Once
	profileOnce sync.Once
	regular     []tg.HelpPeerColorOption
	profile     []tg.HelpPeerColorOption
}{}

func seedWallPapers() []tg.WallPaperClass {
	catalog := appearance.Default()
	out := make([]tg.WallPaperClass, 0, len(catalog.Wallpapers))
	for _, wallpaper := range catalog.Wallpapers {
		out = append(out, seedWallPaper(wallpaper))
	}
	return out
}

// LookupWallPaper resolves a cloud wallpaper from the default seed catalog.
func LookupWallPaper(input tg.InputWallPaperClass) (tg.WallPaperClass, bool) {
	if in, ok := input.(*tg.InputWallPaperNoFile); ok {
		return &tg.WallPaperNoFile{ID: in.ID}, true
	}
	catalog := appearance.Default()
	for _, wallpaper := range catalog.Wallpapers {
		if inputWallPaperMatches(input, wallpaper) {
			return seedWallPaper(wallpaper), true
		}
	}
	return nil, false
}

// LookupWallPapers resolves multiple wallpapers from the default seed catalog.
func LookupWallPapers(inputs []tg.InputWallPaperClass) ([]tg.WallPaperClass, bool) {
	out := make([]tg.WallPaperClass, 0, len(inputs))
	for _, input := range inputs {
		wallpaper, ok := LookupWallPaper(input)
		if !ok {
			return nil, false
		}
		out = append(out, wallpaper)
	}
	return out, true
}

func inputWallPaperMatches(input tg.InputWallPaperClass, wallpaper appearance.Wallpaper) bool {
	switch in := input.(type) {
	case *tg.InputWallPaper:
		return in.ID == wallpaper.ID && in.AccessHash == wallpaper.AccessHash
	case *tg.InputWallPaperSlug:
		return in.Slug != "" && in.Slug == wallpaper.Slug
	default:
		return false
	}
}

func seedWallPaper(in appearance.Wallpaper) tg.WallPaperClass {
	if in.Type == 1 || in.Document.ID == 0 {
		out := &tg.WallPaperNoFile{ID: in.ID}
		out.SetDefault(in.Default)
		out.SetDark(in.Dark)
		out.SetSettings(seedWallPaperSettings(in.Settings))
		return out
	}
	out := &tg.WallPaper{
		ID:         in.ID,
		AccessHash: in.AccessHash,
		Slug:       in.Slug,
		Document:   seedDocument(in.Document),
	}
	out.SetDefault(in.Default)
	out.SetPattern(in.Pattern)
	out.SetDark(in.Dark)
	out.SetSettings(seedWallPaperSettings(in.Settings))
	return out
}

func seedWallPaperSettings(in appearance.WallpaperSettings) tg.WallPaperSettings {
	var out tg.WallPaperSettings
	out.SetBlur(in.Blur)
	out.SetMotion(in.Motion)
	if in.BackgroundColor != 0 {
		out.SetBackgroundColor(in.BackgroundColor)
	}
	if in.SecondBackgroundColor != 0 {
		out.SetSecondBackgroundColor(in.SecondBackgroundColor)
	}
	if in.ThirdBackgroundColor != 0 {
		out.SetThirdBackgroundColor(in.ThirdBackgroundColor)
	}
	if in.FourthBackgroundColor != 0 {
		out.SetFourthBackgroundColor(in.FourthBackgroundColor)
	}
	if in.Intensity != 0 {
		out.SetIntensity(in.Intensity)
	}
	if in.Rotation != 0 {
		out.SetRotation(in.Rotation)
	}
	return out
}

func seedDocument(in appearance.Document) tg.DocumentClass {
	if in.ID == 0 {
		return &tg.DocumentEmpty{}
	}
	return &tg.Document{
		ID:            in.ID,
		AccessHash:    in.AccessHash,
		Date:          in.Date,
		MimeType:      in.MimeType,
		Size:          in.Size,
		Thumbs:        seedPhotoSizes(in.Thumbs),
		DCID:          appearanceSeedDCID,
		Attributes:    seedDocumentAttributes(in.Attributes),
		FileReference: nil,
	}
}

func seedPhotoSizes(in []appearance.PhotoSize) []tg.PhotoSizeClass {
	out := make([]tg.PhotoSizeClass, 0, len(in))
	for _, size := range in {
		switch size.Kind {
		case "size":
			if size.Type != "" && size.W > 0 && size.H > 0 && size.Size > 0 {
				out = append(out, &tg.PhotoSize{Type: size.Type, W: size.W, H: size.H, Size: size.Size})
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func seedDocumentAttributes(in []appearance.DocumentAttribute) []tg.DocumentAttributeClass {
	out := make([]tg.DocumentAttributeClass, 0, len(in))
	for _, attr := range in {
		switch attr.Kind {
		case "image_size":
			out = append(out, &tg.DocumentAttributeImageSize{W: attr.W, H: attr.H})
		case "filename":
			if attr.FileName != "" {
				out = append(out, &tg.DocumentAttributeFilename{FileName: attr.FileName})
			}
		}
	}
	return out
}

func seedPeerColorOptions(profile bool) []tg.HelpPeerColorOption {
	if profile {
		peerColorOptionsCache.profileOnce.Do(func() {
			peerColorOptionsCache.profile = buildSeedPeerColorOptions(true)
		})
		return clonePeerColorOptions(peerColorOptionsCache.profile)
	}
	peerColorOptionsCache.regularOnce.Do(func() {
		peerColorOptionsCache.regular = buildSeedPeerColorOptions(false)
	})
	return clonePeerColorOptions(peerColorOptionsCache.regular)
}

func buildSeedPeerColorOptions(profile bool) []tg.HelpPeerColorOption {
	catalog := appearance.Default()
	source := catalog.PeerColors
	if profile {
		source = catalog.PeerProfileColors
	}
	out := make([]tg.HelpPeerColorOption, 0, len(source))
	for _, color := range source {
		option := tg.HelpPeerColorOption{ColorID: color.ID}
		option.SetHidden(color.Hidden)
		channelMin := boundedPeerColorMinLevel(color.ChannelMinLevel)
		if channelMin > 0 {
			option.SetChannelMinLevel(channelMin)
		}
		groupMin := boundedPeerColorMinLevel(color.GroupMinLevel)
		if groupMin == 0 && profile {
			groupMin = channelMin
		}
		if groupMin > 0 {
			option.SetGroupMinLevel(groupMin)
		}
		if colors := seedPeerColorSet(color.Colors); colors != nil {
			option.SetColors(colors)
		}
		if colors := seedPeerColorSet(color.DarkColors); colors != nil {
			option.SetDarkColors(colors)
		}
		out = append(out, option)
	}
	return out
}

func clonePeerColorOptions(in []tg.HelpPeerColorOption) []tg.HelpPeerColorOption {
	if len(in) == 0 {
		return nil
	}
	out := make([]tg.HelpPeerColorOption, len(in))
	for i := range in {
		out[i] = in[i]
		if in[i].Colors != nil {
			out[i].Colors = clonePeerColorSet(in[i].Colors)
		}
		if in[i].DarkColors != nil {
			out[i].DarkColors = clonePeerColorSet(in[i].DarkColors)
		}
	}
	return out
}

func clonePeerColorSet(in tg.HelpPeerColorSetClass) tg.HelpPeerColorSetClass {
	switch set := in.(type) {
	case *tg.HelpPeerColorSet:
		return &tg.HelpPeerColorSet{Colors: append([]int(nil), set.Colors...)}
	case *tg.HelpPeerColorProfileSet:
		return &tg.HelpPeerColorProfileSet{
			PaletteColors: append([]int(nil), set.PaletteColors...),
			BgColors:      append([]int(nil), set.BgColors...),
			StoryColors:   append([]int(nil), set.StoryColors...),
		}
	default:
		return in
	}
}

func boundedPeerColorMinLevel(level int) int {
	if level <= 0 {
		return 0
	}
	if level > maxPeerColorBoostLevel {
		return maxPeerColorBoostLevel
	}
	return level
}

func seedPeerColorID(id int, profile bool) (bool, bool) {
	catalog := appearance.Default()
	source := catalog.PeerColors
	if profile {
		source = catalog.PeerProfileColors
	}
	if len(source) == 0 {
		return false, false
	}
	for _, color := range source {
		if color.ID == id {
			return true, true
		}
	}
	return false, true
}

func seedPeerColorSet(in *appearance.ColorSet) tg.HelpPeerColorSetClass {
	if in == nil {
		return nil
	}
	if len(in.PaletteColors) > 0 || len(in.BgColors) > 0 || len(in.StoryColors) > 0 {
		return &tg.HelpPeerColorProfileSet{
			PaletteColors: append([]int(nil), in.PaletteColors...),
			BgColors:      append([]int(nil), in.BgColors...),
			StoryColors:   append([]int(nil), in.StoryColors...),
		}
	}
	if len(in.Colors) > 0 {
		return &tg.HelpPeerColorSet{Colors: append([]int(nil), in.Colors...)}
	}
	return nil
}
