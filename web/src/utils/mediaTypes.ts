import {
  FilmIcon,
  TvIcon,
  MusicalNoteIcon,
  BookOpenIcon,
  FolderIcon,
  CommandLineIcon,
  CpuChipIcon,
  NoSymbolIcon,
  SparklesIcon
} from "@heroicons/react/24/outline";

export type MediaType = 'Movie' | 'Show' | 'Anime' | 'Game' | 'Book' | 'Audio' | 'Application' | 'Adult' | 'Other';

export function getMediaType(category: string): MediaType {
  const lowercase = category.toLowerCase();
  
  // Anime specific detection
  if (lowercase.includes('anime') || lowercase.includes('ona') || lowercase.includes('ova')) {
    return 'Anime';
  }
  
  // Movies
  if (lowercase.includes('movie') || lowercase.includes('3d') || lowercase.includes('bluray')) {
    return 'Movie';
  }
  
  // TV Shows
  if (lowercase.includes('tv') || lowercase.includes('show') || 
      lowercase.includes('episode') || lowercase.includes('season')) {
    return 'Show';
  }
  
  // Games
  if (lowercase.includes('game') || lowercase.includes('nintendo') || 
      lowercase.includes('playstation') || lowercase.includes('visual novel')) {
    return 'Game';
  }
  
  // Books and Reading Material
  if (lowercase.includes('book') || lowercase.includes('manga') || 
      lowercase.includes('comic') || lowercase.includes('magazine') ||
      lowercase.includes('novel')) {
    return 'Book';
  }
  
  // Audio content
  if (lowercase.includes('audio') || lowercase.includes('music') || 
      lowercase.includes('flac')) {
    return 'Audio';
  }
  
  // Applications and Software
  if (lowercase.includes('app') || lowercase.includes('software')) {
    return 'Application';
  }
  
  // Adult content
  if (lowercase.includes('xxx') || lowercase.includes('adult')) {
    return 'Adult';
  }
  
  return 'Other';
}

export const mediaTypeIcons = {
  Movie: FilmIcon,
  Show: TvIcon,
  Anime: SparklesIcon,
  Game: CommandLineIcon,
  Book: BookOpenIcon,
  Audio: MusicalNoteIcon,
  Application: CpuChipIcon,
  Adult: NoSymbolIcon,
  Other: FolderIcon,
} as const;

// Add this type to handle the HeroIcon component type
type HeroIcon = typeof mediaTypeIcons[keyof typeof mediaTypeIcons];

// Update the return type of getMediaTypeIcon
export function getMediaTypeIcon(type: MediaType): HeroIcon {
  return mediaTypeIcons[type] || mediaTypeIcons.Other;
}