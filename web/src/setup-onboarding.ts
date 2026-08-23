export type SetupOnboardingLocale = "zh" | "zht" | "en" | "ja" | "ko";

export const SPATIALEMU_FOLIOSPACE_URL = "https://spatialemu.com/foliospace/";
export const SPATIALEMU_FOLIOSPACE_ZH_CN_URL = "https://spatialemu.com/zh-cn/foliospace/";
export const SPATIALEMU_CONNECTION_GUIDE_URL = "https://spatialemu.com/guides/foliospace-connection/";
export const SPATIALEMU_CONNECTION_GUIDE_ZH_CN_URL = "https://spatialemu.com/zh-cn/guides/foliospace-connection/";

export type SetupOnboardingLinks = {
  learnMore: string;
  setupGuide: string;
};

export function setupOnboardingLinks(locale: SetupOnboardingLocale): SetupOnboardingLinks {
  const simplifiedChinese = locale === "zh" || locale === "zht";
  return {
    learnMore: simplifiedChinese ? SPATIALEMU_FOLIOSPACE_ZH_CN_URL : SPATIALEMU_FOLIOSPACE_URL,
    setupGuide: simplifiedChinese ? SPATIALEMU_CONNECTION_GUIDE_ZH_CN_URL : SPATIALEMU_CONNECTION_GUIDE_URL,
  };
}

export type SetupOnboardingCopy = {
  title: string;
  freshSummary: string;
  existingSummary: string;
  language: string;
  optionalTitle: string;
  optionalBody: string;
  hostRequirement: string;
  configurationRequirement: string;
  setupGuide: string;
  learnMore: string;
  newToken: string;
  existingToken: string;
  tokenPlaceholder: string;
  libraryName: string;
  libraryNamePlaceholder: string;
  containerPath: string;
  containerPathPlaceholder: string;
  assetType: string;
  scanWorkers: string;
  advancedCatalog: string;
  catalogAutomation: string;
  coverLookup: string;
  policyFiles: string;
  mountHint: string;
  nextStepPrefix: string;
  nextStepSuffix: string;
  initialize: string;
  saveSetup: string;
  authPrompt: string;
  accessToken: string;
  unlock: string;
};

export function setupOnboardingCopy(locale: SetupOnboardingLocale): SetupOnboardingCopy {
  if (locale === "zh") {
    return {
      title: "设置 FolioSpace Library",
      freshSummary: "创建访问 token，并选择 Docker 容器内已挂载的第一个媒体目录。",
      existingSummary: "确认访问 token，并继续使用数据库里已有的媒体目录。",
      language: "语言",
      optionalTitle: "在 SpatialEMU 中使用？FolioSpace 是可选服务。",
      optionalBody: "没有 FolioSpace，SpatialEMU 也可以直接打开本地文件。只有需要自托管目录时才需要运行此服务。",
      hostRequirement: "不需要 NAS，也不需要写代码。可在装有 Docker/Compose 的 Mac、Windows 或 Linux 电脑，或兼容 NAS 上运行。",
      configurationRequirement: "你需要 Docker、媒体目录、此服务的 URL，以及一个访问 token。",
      setupGuide: "SpatialEMU 连接指南",
      learnMore: "了解 FolioSpace",
      newToken: "创建访问 token",
      existingToken: "当前访问 token",
      tokenPlaceholder: "至少 8 个字符；请妥善保存",
      libraryName: "媒体目录名称",
      libraryNamePlaceholder: "Games / Books / Comics",
      containerPath: "容器内路径",
      containerPathPlaceholder: "/games",
      assetType: "媒体类型",
      scanWorkers: "扫描 Worker",
      advancedCatalog: "高级游戏目录选项（可选）",
      catalogAutomation: "扫描完成后自动分类并生成可用的启动配置",
      coverLookup: "允许从 Libretro Thumbnails 补全缺失封面",
      policyFiles: "高级策略文件",
      mountHint: "如果没有看到媒体目录，请先在 Docker Compose 中挂载宿主机目录，例如 /path/to/Games:/games:ro。这里请选择容器内路径，如 /games。",
      nextStepPrefix: "完成后，在 SpatialEMU 的 FolioSpace 连接界面输入此服务 URL：",
      nextStepSuffix: "再输入同一个 token，选择“连接”，并确认目录能够加载。",
      initialize: "完成设置",
      saveSetup: "保存设置",
      authPrompt: "输入 FolioSpace 访问 token。",
      accessToken: "访问 token",
      unlock: "解锁",
    };
  }
  if (locale === "zht") {
    return {
      title: "設定 FolioSpace Library",
      freshSummary: "建立存取 token，並選擇 Docker 容器內已掛載的第一個媒體目錄。",
      existingSummary: "確認存取 token，並繼續使用資料庫內已有的媒體目錄。",
      language: "語言",
      optionalTitle: "在 SpatialEMU 中使用？FolioSpace 是選用服務。",
      optionalBody: "沒有 FolioSpace，SpatialEMU 也可以直接開啟本機檔案。只有需要自託管目錄時才需要執行此服務。",
      hostRequirement: "不需要 NAS，也不需要寫程式。可在裝有 Docker/Compose 的 Mac、Windows 或 Linux 電腦，或相容 NAS 上執行。",
      configurationRequirement: "你需要 Docker、媒體目錄、此服務的 URL，以及一個存取 token。",
      setupGuide: "SpatialEMU 連線指南",
      learnMore: "瞭解 FolioSpace",
      newToken: "建立存取 token",
      existingToken: "目前存取 token",
      tokenPlaceholder: "至少 8 個字元；請妥善保存",
      libraryName: "媒體目錄名稱",
      libraryNamePlaceholder: "Games / Books / Comics",
      containerPath: "容器內路徑",
      containerPathPlaceholder: "/games",
      assetType: "媒體類型",
      scanWorkers: "掃描 Worker",
      advancedCatalog: "進階遊戲目錄選項（選用）",
      catalogAutomation: "掃描完成後自動分類並產生可用的啟動設定",
      coverLookup: "允許從 Libretro Thumbnails 補齊缺少的封面",
      policyFiles: "進階策略檔案",
      mountHint: "如果沒有看到媒體目錄，請先在 Docker Compose 中掛載主機目錄，例如 /path/to/Games:/games:ro。這裡請選擇容器內路徑，例如 /games。",
      nextStepPrefix: "完成後，在 SpatialEMU 的 FolioSpace 連線畫面輸入此服務 URL：",
      nextStepSuffix: "再輸入相同的 token，選擇「連線」，並確認目錄可以載入。",
      initialize: "完成設定",
      saveSetup: "儲存設定",
      authPrompt: "輸入 FolioSpace 存取 token。",
      accessToken: "存取 token",
      unlock: "解鎖",
    };
  }
  return {
    title: "Set up FolioSpace Library",
    freshSummary: "Create an access token and choose the first media folder mounted inside this Docker container.",
    existingSummary: "Confirm the access token and keep using the media folders already stored in this library.",
    language: "Language",
    optionalTitle: "Using SpatialEMU? FolioSpace is optional.",
    optionalBody: "SpatialEMU can open local files directly without this server. Run FolioSpace only when you want a self-hosted catalog.",
    hostRequirement: "A NAS and coding are not required. Run Docker/Compose on a Mac, Windows or Linux computer, or a compatible NAS.",
    configurationRequirement: "You need Docker, your media folders, this service URL, and an access token.",
    setupGuide: "SpatialEMU setup guide",
    learnMore: "Learn about FolioSpace",
    newToken: "Create an access token",
    existingToken: "Current access token",
    tokenPlaceholder: "At least 8 characters; keep it safe",
    libraryName: "Media folder name",
    libraryNamePlaceholder: "Games / Books / Comics",
    containerPath: "Container path",
    containerPathPlaceholder: "/games",
    assetType: "Media type",
    scanWorkers: "Scan workers",
    advancedCatalog: "Advanced game catalog options (optional)",
    catalogAutomation: "Classify games and build available launch profiles after each scan",
    coverLookup: "Allow Libretro Thumbnails to fill missing covers",
    policyFiles: "Advanced policy files",
    mountHint: "If a media folder is missing, mount its host path in Docker Compose first, for example /path/to/Games:/games:ro. Choose the container path here, such as /games.",
    nextStepPrefix: "After setup, enter this service URL in SpatialEMU's FolioSpace connection screen:",
    nextStepSuffix: "Enter the same token, select Connect, and confirm that the catalog loads.",
    initialize: "Finish setup",
    saveSetup: "Save setup",
    authPrompt: "Enter your FolioSpace access token.",
    accessToken: "Access token",
    unlock: "Unlock",
  };
}
