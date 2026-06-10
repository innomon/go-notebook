import { enUS } from './en-US';
import { bnIN } from './bn-IN';

export const resources = {
  'en-US': { translation: enUS },
  'bn-IN': { translation: bnIN },
} as const;

export type TranslationKeys = typeof enUS;

export type LanguageCode = 'en-US' | 'bn-IN';

export type Language = {
  code: LanguageCode;
  label: string;
};

export const languages: Language[] = [
  { code: 'en-US', label: 'English' },
  { code: 'bn-IN', label: 'বাংলা' },
];

export { enUS, bnIN };
