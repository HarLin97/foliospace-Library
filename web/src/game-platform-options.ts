type GamePlatformCollection = {
  title: string;
  collectionType: string;
  primaryType: string;
  bookCount: number;
};

export type GamePlatformFilterOption = {
  value: string;
  label: string;
  count: number;
};

export function gamePlatformFilterOptions(collections: GamePlatformCollection[]): GamePlatformFilterOption[] {
  return collections
    .filter((item) => item.collectionType === "game_platform" && item.primaryType === "game")
    .map((item) => {
      const label = gamePlatformCollectionLabel(item.title);
      return {
        value: gamePlatformValueFromLabel(label),
        label,
        count: item.bookCount,
      };
    })
    .filter((item) => item.value !== "");
}

function gamePlatformCollectionLabel(title: string) {
  return title.trim().replace(/^Games\s*\/\s*/i, "").trim();
}

function gamePlatformValueFromLabel(label: string) {
  switch (label.trim().toLowerCase()) {
    case "mega drive":
      return "md";
    case "neo geo":
      return "neogeo";
    case "model 3":
      return "model3";
    case "naomi":
      return "naomi";
    case "mame":
      return "mame";
    case "nintendo 64":
      return "n64";
    case "nec pc-98":
    case "pc-98":
      return "pc98";
    case "32x":
      return "32x";
    default:
      return label.trim().toLowerCase();
  }
}
