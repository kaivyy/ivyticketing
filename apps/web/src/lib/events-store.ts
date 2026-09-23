import { fetchPublicEvents, fetchPublicEvent } from "./api";

export interface MarathonCategory {
  id: string;
  code: string;
  name: string;
  distance: string; // e.g. "42.195 KM"
  distanceKm: number;
  cutoffTime: string; // e.g. "07:00:00"
  minAge: number;
  price: number;
  earlyBirdPrice?: number;
  quota: number;
  registeredCount: number;
  mode: "WAR_QUEUE" | "BALLOT" | "NORMAL" | "PRIORITY_ACCESS" | "WAITLIST_ONLY";
  status: "OPEN" | "WAR_SOON" | "BALLOT_OPEN" | "BALLOT_CLOSED" | "SOLD_OUT" | "COMING_SOON";
  opensAt: string;
  closesAt: string;
  includes: string[];
}

export interface MarathonEvent {
  id: string;
  slug: string;
  name: string;
  tagline: string;
  description: string;
  eventType: string; // "MARATHON", "ROAD_RACE", "TRAIL_RUN"
  status: "draft" | "published" | "archived";
  organizerName: string;
  organizerSlug: string;
  city: string;
  province: string;
  country: string;
  venueName: string;
  venueAddress: string;
  startsAt: string; // ISO date
  flagOffTime: string; // e.g. "04:30 WIB"
  bannerUrl: string;
  logoUrl: string;
  themeColor: string; // Hex e.g. "#ea580c"
  accentColor: string;
  certifiedBy?: string; // "World Athletics & PASI"
  elevationGain: string; // e.g. "120m (Flat PB Course)"
  waterStationsCount: number;
  medicalStationsCount: number;
  categories: MarathonCategory[];
  routeHighlights: string[];
  racepackInfo: {
    dates: string;
    venue: string;
    address: string;
    hours: string;
    requirements: string[];
  };
  schedule: Array<{
    time: string;
    activity: string;
    category?: string;
  }>;
  faqs: Array<{
    q: string;
    a: string;
  }>;
  sponsors: Array<{
    name: string;
    role: string;
  }>;
  customFormConfig?: CustomFormConfig;
  layoutConfig?: MarathonLayoutConfig;
  timingConfig?: {
    provider: string;
    transport?: string;
    syncMode?: string;
    externalRaceId?: string;
    policy?: {
      debounceWindowSeconds?: number;
      cutoffMinutes?: number;
      allowMissingStart?: boolean;
    };
    checkpoints?: Array<{
      code: string;
      name: string;
      checkpointType: string;
      orderIndex: number;
      distanceMeters?: number;
      aliases?: string[];
    }>;
  };
  updatedAt: string;
}

export type MarathonTemplateId =
  | "template-summarecon"
  | "template-tokyo"
  | "template-berlin"
  | "template-borobudur"
  | "template-sydney"
  | "template-bangkok"
  | "template-ironman"
  | "template-baa"
  | "template-custom";

export interface AvailableTemplate {
  id: MarathonTemplateId;
  name: string;
  source: string;
  badge: string;
  badgeClass: string;
  description: string;
  previewBanner: string;
  defaultTheme: "summarecon_neon" | "borobudur_heritage" | "tokyo_platinum" | "berlin_speed" | "custom";
  defaultVariant: "poster" | "split" | "centered" | "cinematic";
}

export const AVAILABLE_TEMPLATES: AvailableTemplate[] = [
  {
    id: "template-summarecon",
    name: "Summarecon Bandung Run Fest",
    source: "summareconbandungrunfest.com",
    badge: "Urban Fest & Neon",
    badgeClass: "bg-emerald-500/20 text-emerald-300 border-emerald-500/30",
    description: "Template festival lari kota dengan tipografi athletic bold, seksi kuning 'New Roads New Possibilities', kartu kategori hitam bernomor 01-04 dengan tombol 'MORE INFO' modal, banner penutup 'The Road Is Finally Yours', dan tekstur kertas warm paper.",
    previewBanner: "/images/events/bandung_runfest.jpg",
    defaultTheme: "summarecon_neon",
    defaultVariant: "poster",
  },
  {
    id: "template-tokyo",
    name: "Tokyo International Marathon",
    source: "marathon.tokyo",
    badge: "Abbott World Major",
    badgeClass: "bg-red-500/20 text-red-300 border-red-500/30",
    description: "Template presisi ala Abbott World Marathon Majors Jepang. Desain kontras tinggi navy & crimson red, tipografi bilingual Tokyo, tabel breakdown kuota ballot 38.000 pelari, timer countdown digital bergaris, dan expo Tokyo Big Sight.",
    previewBanner: "/images/events/tokyo_marathon.jpg",
    defaultTheme: "tokyo_platinum",
    defaultVariant: "centered",
  },
  {
    id: "template-berlin",
    name: "BMW Berlin Speed Marathon",
    source: "bmw-berlin-marathon.com",
    badge: "World Record Fastest",
    badgeClass: "bg-sky-500/20 text-sky-300 border-sky-500/30",
    description: "Template lintasan rekor dunia tercepat. Layout split-screen berdampingan, garis kecepatan BMW Blue, grafik elevasi flat 20m, kategori inline skate/handbike/running, dan klimaks Gerbang Brandenburg KM 41.8.",
    previewBanner: "/images/events/hero_marathon.jpg",
    defaultTheme: "berlin_speed",
    defaultVariant: "split",
  },
  {
    id: "template-borobudur",
    name: "Borobudur Heritage Marathon",
    source: "borobudurmarathon.com",
    badge: "Cultural Heritage",
    badgeClass: "bg-orange-500/20 text-orange-300 border-orange-500/30",
    description: "Template kultural warisan budaya dunia. Latar fajar Candi Borobudur dengan warna terakota & emas antik, tajuk 'Decade of Legacy / Dasa Warsa Warisan', rute rolling hills Menoreh (285m), 15 cheer zone kesenian warga 12 desa, dan Pasar Medang.",
    previewBanner: "/images/events/borobudur_marathon.jpg",
    defaultTheme: "borobudur_heritage",
    defaultVariant: "cinematic",
  },
  {
    id: "template-sydney",
    name: "TCS Sydney Harbour Marathon",
    source: "sydneymarathon.com",
    badge: "Harbour Major",
    badgeClass: "bg-cyan-500/20 text-cyan-300 border-cyan-500/30",
    description: "Template maritim spektakuler Harbour Bridge hingga Sydney Opera House. Desain biru laut pesisir pasifik, pembagian assembly area wave warna (Purple, Green, Orange, High Performance), dan angin sepoi pesisir.",
    previewBanner: "/images/events/hero_marathon.jpg",
    defaultTheme: "berlin_speed",
    defaultVariant: "poster",
  },
  {
    id: "template-bangkok",
    name: "Bangkok Midnight Marathon (BDMS)",
    source: "bkkmarathon.com",
    badge: "Midnight Grand Palace",
    badgeClass: "bg-amber-500/20 text-amber-300 border-amber-500/30",
    description: "Template lari malam tropis Sanam Chai Grand Palace & Jembatan Kabel Rama VIII. Kartu tiket bergaya sobekan sirkular (card-ticket side cutouts), badge miring skew-box (-15 deg), tombol 3D border-bottom, dan tabel jadwal bersayap melengkung dengan aksen Fire Orange & Thai Amber.",
    previewBanner: "/images/events/bangkok_marathon.jpg",
    defaultTheme: "custom",
    defaultVariant: "centered",
  },
  {
    id: "template-ironman",
    name: "IRONMAN 70.3 & Full Triathlon",
    source: "ironman.com",
    badge: "Triathlon World Series",
    badgeClass: "bg-red-600/20 text-red-300 border-red-500/40",
    description: "Template multi-disiplin triathlon resmi standar IRONMAN. Menampilkan split 3 disiplin (Swim 1.9K/3.8K, Bike 90K/180K, Run 21.1K/42.2K) plus transisi T1 & T2, dasbor metrik cuaca dan suhu air, cut-off time bertingkat tiap disiplin, kuota slot World Championship Kona, dan panggung finish karpet merah.",
    previewBanner: "/images/events/ironman_triathlon.jpg",
    defaultTheme: "custom",
    defaultVariant: "poster",
  },
  {
    id: "template-baa",
    name: "Boston Athletic Association (B.A.A.) Marathon",
    source: "baa.org",
    badge: "B.A.A. Abbott Major",
    badgeClass: "bg-blue-900/40 text-yellow-300 border-yellow-400/40",
    description: "Template otentik B.A.A. Boston Marathon dari https://www.baa.org/. Foto besar hero Boylston Street, pita alert registrasi kualifikasi kuning, animasi pita sponsor bergerak tanpa putus (Bank of America, adidas, Abbott, Sam Adams, Honda, Maurten, Shokz, JetBlue, Gatorade), badge unicorn B.A.A., dan tabel standar waktu kualifikasi resmi.",
    previewBanner: "/images/events/boston_marathon.jpg",
    defaultTheme: "custom",
    defaultVariant: "cinematic",
  },
  {
    id: "template-custom",
    name: "Super Custom Studio Template",
    source: "IvyTicketing Engine",
    badge: "Kustom Bebas 100%",
    badgeClass: "bg-purple-500/20 text-purple-300 border-purple-500/30",
    description: "Template fleksibel penuh tanpa batas. Anda bebas menentukan posisi judul (kiri/tengah/kanan), varian hero, skema warna hex mandiri, tombol CTA, visibilitas seksi, dan menambahkan blok konten apa pun.",
    previewBanner: "/images/events/hero_marathon.jpg",
    defaultTheme: "custom",
    defaultVariant: "poster",
  },
];

export interface EventNavbarConfig {
  brandBadgeText?: string;
  brandBadgeBg?: string;
  brandBadgeColor?: string;
  brandIconUrl?: string;
  brandTitle?: string;
  brandSubtitle?: string;
}

export interface BaaLayoutConfig {
  presentedByText?: string;
  topAlertText?: string;
  topAlertLinkText?: string;
  topAlertLinkUrl?: string;
  unicornBadgeText?: string;
  heroLargePhotoUrl?: string;
  finishLinePhotoUrl?: string;
  editionText?: string;
  sloganText?: string;
}

export type IronmanPresetIconType =
  | "swim"
  | "bike"
  | "run"
  | "sun"
  | "snowflake"
  | "water"
  | "thermometer"
  | "wind"
  | "mountain"
  | "trophy"
  | "medal"
  | "flag"
  | "timer"
  | "heart";

export interface IronmanMetricItem {
  id: string;
  label: string;
  value: string;
  iconType: "preset" | "custom";
  presetIcon?: IronmanPresetIconType;
  customIconUrl?: string;
}

export interface IronmanSealItem {
  id: string;
  badgeTag: string;
  title: string;
  subtitle: string;
  iconUrl?: string;
  colorTheme?: "blue" | "gold" | "red" | "dark" | "emerald";
}

export interface IronmanWhyCardItem {
  id: string;
  iconEmoji?: string;
  imageUrl?: string;
  title: string;
  description: string;
}

export interface IronmanLayoutConfig {
  topRibbonText?: string;
  tagBadge?: string;
  heroCtaPrimaryText?: string;
  heroCtaPrimaryLink?: string;
  heroCtaSecondaryText?: string;
  heroCtaSecondaryLink?: string;
  metrics?: IronmanMetricItem[];
  swimMetric?: string;
  bikeMetric?: string;
  runMetric?: string;
  airTempMetric?: string;
  lowAirTempMetric?: string;
  waterTempMetric?: string;
  konaSlots?: string;
  splendorTitle?: string;
  splendorDescription?: string;
  youtubeUrl?: string;
  videoThumbnailUrl?: string;
  videoBadgeText?: string;
  seals?: IronmanSealItem[];
  whySectionTagline?: string;
  whySectionTitle?: string;
  whySectionSubtitle?: string;
  whyBgColor?: string;
  whyCards?: IronmanWhyCardItem[];
  showStickyMobileCta?: boolean;
  stickyMobileTitle?: string;
  stickyMobileSubtitle?: string;
  stickyMobileBtnText?: string;
  stickyMobileBtnLink?: string;
}

export interface BangkokLayoutConfig {
  runnerPhotoUrl?: string;
  midnightCallout?: string;
  topRibbonText?: string;
  skewBadgeText?: string;
}

export interface TokyoLayoutConfig {
  kanjiBadge?: string;
  taglineSub?: string;
  topRibbonText?: string;
}

export interface BerlinLayoutConfig {
  speedRecordNote?: string;
  topRibbonText?: string;
}

export interface BorobudurLayoutConfig {
  heritageHeadline?: string;
  cheerZonesText?: string;
}

export interface SummareconLayoutConfig {
  neonHeadline?: string;
  closingPosterText?: string;
}

export interface EventQueueConfig {
  enabled?: boolean;
  mode?: "auto" | "bypass" | "always_queue";
  algorithm?: "fifo" | "randomized";
  trafficThreshold?: number;
  activeWaitingCount?: number;
  releaseRatePerMinute?: number;
}

export interface MarathonLayoutConfig {
  templateId: MarathonTemplateId;
  heroAlignment: "left" | "center" | "right";
  heroVariant: "centered" | "split" | "poster" | "cinematic";
  themeStyle: "summarecon_neon" | "borobudur_heritage" | "tokyo_platinum" | "berlin_speed" | "custom";
  customThemePrimary?: string;
  customThemeAccent?: string;
  paperTexture: boolean;
  showTicker: boolean;
  tickerText: string;
  showCountdown: boolean;
  showCategories: boolean;
  showRoute: boolean;
  showRpc: boolean;
  showSchedule: boolean;
  showFaq: boolean;
  showSponsors: boolean;
  showCustomBlocks: boolean;
  categorySectionTitle: string;
  categorySectionSubtitle: string;
  routeSectionTitle: string;
  routeSectionSubtitle?: string;
  rpcSectionTitle: string;
  rpcSectionSubtitle?: string;
  scheduleSectionTitle: string;
  scheduleSectionSubtitle?: string;
  ctaButtonText: string;
  buttonShape?: "pill" | "rounded" | "square";
  customBlocks?: Array<{
    id: string;
    title: string;
    content: string;
    badge?: string;
    style?: "card" | "banner" | "notice";
  }>;
  ironmanConfig?: IronmanLayoutConfig;
  bangkokConfig?: BangkokLayoutConfig;
  tokyoConfig?: TokyoLayoutConfig;
  berlinConfig?: BerlinLayoutConfig;
  borobudurConfig?: BorobudurLayoutConfig;
  summareconConfig?: SummareconLayoutConfig;
  baaConfig?: BaaLayoutConfig;
  navbarConfig?: EventNavbarConfig;
  queueConfig?: EventQueueConfig;
}

export const DEFAULT_LAYOUT_CONFIG: MarathonLayoutConfig = {
  templateId: "template-summarecon",
  heroAlignment: "center",
  heroVariant: "poster",
  themeStyle: "summarecon_neon",
  paperTexture: false,
  showTicker: true,
  tickerText: "NEW ROADS · NEW POSSIBILITIES · WORLD ATHLETICS CERTIFIED COURSE · FLAT & FAST PB ROUTE · OFFICIAL TIMING RFID",
  showCountdown: true,
  showCategories: true,
  showRoute: true,
  showRpc: true,
  showSchedule: true,
  showFaq: true,
  showSponsors: true,
  showCustomBlocks: true,
  categorySectionTitle: "PILIHAN KATEGORI LOMBA & BIB",
  categorySectionSubtitle: "Pilih jarak tantangan lari Anda. Setiap kategori dilengkapi paket resmi peserta, medali finisher logam cor, dan catatan waktu chip RFID.",
  routeSectionTitle: "LINTASAN & SPESIFIKASI RUTE",
  routeSectionSubtitle: "Profil elevasi jalan raya steril dengan standar pengukuran akurat dan titik hidrasi terstruktur.",
  rpcSectionTitle: "PENGAMBILAN RACEPACK (RPC)",
  rpcSectionSubtitle: "Jadwal dan persyaratan verifikasi identitas resmi sebelum memasuki arena perlombaan.",
  scheduleSectionTitle: "RANGKAIAN JADWAL HARI H",
  scheduleSectionSubtitle: "Urutan waktu flag-off, cutoff waktu per kategori, dan seremoni panggung juara.",
  ctaButtonText: "DAFTAR SEKARANG",
  buttonShape: "rounded",
  customBlocks: [
    {
      id: "block-shuttle",
      title: "Layanan Shuttle Bus Resmi & Kantong Parkir",
      badge: "INFO LOGISTIK",
      style: "card",
      content: "Panitia menyediakan shuttle bus gratis dari stasiun dan terminal transit menuju race village mulai pukul 03:00 WIB hingga 11:30 WIB. Kantong parkir resmi tersedia di area stadion dan mall rekanan dengan stiker pelari resmi.",
    },
    {
      id: "block-rules",
      title: "Aturan Lomba, Cut-Off Time & Diskualifikasi",
      badge: "ATURAN LOMBA",
      style: "notice",
      content: "Peserta wajib mengenakan nomor BIB di bagian dada depan. Penggunaan sepeda pengawal non-resmi atau pemotongan rute otomatis mengakibatkan diskualifikasi seketika (DQ) dari pencatatan waktu RFID.",
    },
  ],
  queueConfig: {
    enabled: true,
    mode: "auto",
    algorithm: "fifo",
    trafficThreshold: 50,
    activeWaitingCount: 0,
    releaseRatePerMinute: 60,
  },
};

export interface CustomFormConfig {
  requireIdNumber: boolean;
  requireBloodType: boolean;
  requireEmergencyContact: boolean;
  requireJerseySize: boolean;
  requireBibName: boolean;
  requireMedicalConditions: boolean;
  requireClubName: boolean;
  requireEstimatedFinishTime: boolean;
  requireProvinceCity?: boolean;
}

export const DEFAULT_FORM_CONFIG: CustomFormConfig = {
  requireIdNumber: true,
  requireBloodType: true,
  requireEmergencyContact: true,
  requireJerseySize: true,
  requireBibName: true,
  requireMedicalConditions: true,
  requireClubName: false,
  requireEstimatedFinishTime: false,
  requireProvinceCity: true,
};

export const GENERATED_IMAGE_PRESETS = [
  {
    id: "bangkok",
    name: "Bangkok Midnight Grand Palace",
    url: "/images/events/bangkok_marathon.jpg",
    description: "Pesona lari malam Bangkok Marathon melintasi Jembatan Rama VIII dan kemegahan Grand Palace",
  },
  {
    id: "ironman",
    name: "IRONMAN Triathlon Championship",
    url: "/images/events/ironman_triathlon.jpg",
    description: "Aksi balap sepeda time-trial triathlon pesisir samudra dan karpet merah finish line IRONMAN",
  },
  {
    id: "borobudur",
    name: "Borobudur Scenic Heritage",
    url: "/images/events/borobudur_marathon.jpg",
    description: "Pelari marathon di lanskap asri dengan siluet Candi Borobudur dan fajar keemasan",
  },
  {
    id: "bandung",
    name: "Summarecon Bandung Run Fest",
    url: "/images/events/bandung_runfest.jpg",
    description: "Festival lari perkotaan meriah di boulevard modern Bandung bersama ribuan pelari",
  },
  {
    id: "tokyo",
    name: "Tokyo International Marathon",
    url: "/images/events/tokyo_marathon.jpg",
    description: "Start line megah kota metropolitan Tokyo dengan panggung atletik presisi tinggi",
  },
  {
    id: "hero",
    name: "Dawn Start Line Marathon",
    url: "/images/events/hero_marathon.jpg",
    description: "Ribuan pelari di garis start fajar pagi bersiap menuntaskan 42.195 km",
  },
];

export const SEED_MARATHON_EVENTS: MarathonEvent[] = [
  {
    id: "borobudur-heritage-marathon-2026",
    slug: "borobudur-heritage-marathon-2026",
    name: "Borobudur Heritage Marathon 2026",
    tagline: "Menembus Batas, Menyatu dengan Warisan Budaya Dunia",
    description:
      "Ajang marathon warisan budaya terbesar di Indonesia. Menghadirkan lintasan asri Magelang dengan pemandangan megah Candi Borobudur, perbukitan Menoreh, dan keramahan sambutan warga pedesaan di sepanjang rute 42.195 km.",
    eventType: "MARATHON",
    status: "published",
    organizerName: "Borobudur Marathon Foundation",
    organizerSlug: "borobudur-heritage",
    city: "Magelang",
    province: "Jawa Tengah",
    country: "Indonesia",
    venueName: "Taman Lumbini, Kompleks Candi Borobudur",
    venueAddress: "Jl. Badrawati, Kawasan Candi Borobudur, Magelang, Jawa Tengah 56553",
    startsAt: "2026-11-15T04:30:00+07:00",
    flagOffTime: "04:30 WIB",
    bannerUrl: "/images/events/borobudur_marathon.jpg",
    logoUrl: "/images/events/borobudur_marathon.jpg",
    themeColor: "#ea580c",
    accentColor: "#f59e0b",
    certifiedBy: "World Athletics & PASI",
    elevationGain: "285m (Scenic Rolling Hills)",
    waterStationsCount: 18,
    medicalStationsCount: 12,
    categories: [
      {
        id: "cat-bhm-fm",
        code: "FM42K",
        name: "Full Marathon",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "07:00:00",
        minAge: 18,
        price: 850000,
        earlyBirdPrice: 750000,
        quota: 3000,
        registeredCount: 2650,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: "2026-09-01T08:00:00+07:00",
        closesAt: "2026-10-15T23:59:59+07:00",
        includes: [
          "Jersey Running Resmi",
          "Jersey Finisher Khusus FM",
          "Medali Finisher Eksklusif Logam Cor",
          "BIB Number dengan RFID Timing Chip",
          "Asuransi Kecelakaan Diri",
          "Akses Refreshment & Recovery Zone",
        ],
      },
      {
        id: "cat-bhm-hm",
        code: "HM21K",
        name: "Half Marathon",
        distance: "21.097 KM",
        distanceKm: 21.097,
        cutoffTime: "03:45:00",
        minAge: 17,
        price: 650000,
        earlyBirdPrice: 550000,
        quota: 4500,
        registeredCount: 4200,
        mode: "WAR_QUEUE",
        status: "WAR_SOON",
        opensAt: "2026-09-20T10:00:00+07:00",
        closesAt: "2026-10-25T23:59:59+07:00",
        includes: [
          "Jersey Running Resmi",
          "Medali Finisher Eksklusif",
          "BIB Number dengan RFID Timing Chip",
          "Asuransi Perlindungan Peserta",
          "Refreshment Stasiun Setiap 2.5 KM",
        ],
      },
      {
        id: "cat-bhm-10k",
        code: "10K",
        name: "10K Challenge",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:00:00",
        minAge: 15,
        price: 450000,
        earlyBirdPrice: 400000,
        quota: 3500,
        registeredCount: 3100,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: "2026-09-10T08:00:00+07:00",
        closesAt: "2026-10-30T23:59:59+07:00",
        includes: [
          "Jersey Running Resmi",
          "Medali Finisher Logam",
          "BIB Number dengan RFID Chip",
          "Goodie Bag Sponsor",
        ],
      },
    ],
    routeHighlights: [
      "Gerbang Utama Candi Borobudur (Start & Finish)",
      "Jalan Pedesaan Wanurejo & Balkondes Tradisional",
      "Pemandangan Pegunungan Menoreh & Kali Progo",
      "Sambutan Gamelan & Kesenian Warga di 15 Titik Cheer Zone",
    ],
    racepackInfo: {
      dates: "13 - 14 November 2026",
      venue: "Grand Artos Hotel & Convention, Magelang",
      address: "Jl. Mayjen Bambang Soegeng No.1, Magelang",
      hours: "10:00 - 20:00 WIB",
      requirements: [
        "E-KTP asli atau Paspor bagi WNA",
        "E-Mail Konfirmasi Pendaftaran & QR Code",
        "Surat Keterangan Sehat (Wajib untuk kategori 42K)",
        "Surat Kuasa bermaterai jika diwakilkan",
      ],
    },
    schedule: [
      { time: "03:00 WIB", activity: "Race Village & Bag Drop Open" },
      { time: "04:00 WIB", activity: "Line Up Start Corrals Full Marathon" },
      { time: "04:30 WIB", activity: "Flag-Off Full Marathon (42.195K)", category: "FM42K" },
      { time: "05:15 WIB", activity: "Flag-Off Half Marathon (21.1K)", category: "HM21K" },
      { time: "06:00 WIB", activity: "Flag-Off 10K Challenge", category: "10K" },
      { time: "08:00 WIB", activity: "Cut-off Time 10K" },
      { time: "09:00 WIB", activity: "Cut-off Time Half Marathon" },
      { time: "10:30 WIB", activity: "Upacara Penyerahan Hadiah & Podium" },
      { time: "11:30 WIB", activity: "Cut-off Time Full Marathon (Finish Line Closed)" },
    ],
    faqs: [
      {
        q: "Bagaimana cara kerja undian sistem Ballot?",
        a: "Peserta mendaftar gratis pada periode ballot. Sistem secara acak memilih peserta terpilih dan notifikasi pembayaran dikirim via email serta WhatsApp untuk diselesaikan dalam 48 jam.",
      },
      {
        q: "Apakah rute memiliki sertifikasi resmi?",
        a: "Ya, rute Borobudur Heritage Marathon telah diukur dan disertifikasi resmi oleh PASI dan World Athletics Grade A.",
      },
      {
        q: "Apakah tersedia fasilitas penitipan tas (Baggage Drop)?",
        a: "Ya, penitipan tas resmi dibuka pukul 03:00 WIB di Taman Lumbini khusus tas racepack resmi yang ditempel stiker nomor BIB.",
      },
    ],
    sponsors: [
      { name: "Bank Jateng", role: "Title Sponsor" },
      { name: "Mizuno", role: "Official Apparel & Footwear" },
      { name: "Pocari Sweat", role: "Official Isotonic Hydration" },
      { name: "Garmin", role: "Official Timing Partner" },
    ],
    layoutConfig: {
      templateId: "template-borobudur",
      heroAlignment: "center",
      heroVariant: "cinematic",
      themeStyle: "borobudur_heritage",
      paperTexture: false,
      showTicker: true,
      tickerText: "DECADE OF LEGACY · DASA WARSA WARISAN · JALUR CANDI BERSEJARAH BOROBUDUR · PESONA PERBUKITAN MENOREH · WORLD ATHLETICS CERTIFIED",
      showCountdown: true,
      showCategories: true,
      showRoute: true,
      showRpc: true,
      showSchedule: true,
      showFaq: true,
      showSponsors: true,
      showCustomBlocks: true,
      categorySectionTitle: "KATEGORI LOMBA & SISTEM BALLOT",
      categorySectionSubtitle: "Pilihan jarak lari warisan budaya dunia. Sistem pendaftaran undian adil dan transparan bagi pelari nasional dan internasional.",
      routeSectionTitle: "LINTASAN & PROFIL ELEVASI MENOREH",
      routeSectionSubtitle: "Rute rolling hills asri melewati 12 desa tradisional Magelang dengan dukungan 15 cheer zones warga lokal.",
      rpcSectionTitle: "PENGAMBILAN RACEPACK (RPC)",
      rpcSectionSubtitle: "Expo perlengkapan lomba dan validasi identitas resmi di kompleks Candi Borobudur.",
      scheduleSectionTitle: "RANGKAIAN JADWAL HARI H",
      scheduleSectionSubtitle: "Jadwal flag-off fajar dan batas waktu cut-off time resmi perlombaan.",
      ctaButtonText: "DAFTAR SEKARANG",
      customBlocks: [
        {
          id: "block-shuttle",
          title: "Layanan Shuttle Bus Magelang & Yogyakarta",
          badge: "INFO LOGISTIK",
          style: "card",
          content: "Tersedia bus shuttle resmi dari Bandara YIA, Stasiun Tugu Yogyakarta, dan Terminal Tidar Magelang menuju Race Village Taman Lumbini mulai H-1 hingga Hari H.",
        },
        {
          id: "block-culture",
          title: "Cheering Zone Seni Musik Warga 12 Desa",
          badge: "WARISAN BUDAYA",
          style: "banner",
          content: "Nikmati alunan gamelan tradisional, tarian Dayakan, dan sorak-sorai hangat ribuan warga lokal di setiap 2 kilometer sepanjang rute perdesaan Borobudur.",
        },
      ],
      queueConfig: {
        enabled: true,
        mode: "auto",
        trafficThreshold: 50,
        activeWaitingCount: 0,
        releaseRatePerMinute: 60,
      },
    },
    updatedAt: new Date().toISOString(),
  },
  {
    id: "summarecon-bandung-runfest-2026",
    slug: "summarecon-bandung-runfest-2026",
    name: "Summarecon Bandung Run Fest 2026",
    tagline: "Festival Lari Penuh Warna di Kota Kembang",
    description:
      "Festival lari keluarga dan komunitas paling bergengsi di Bandung Timur. Menjelajahi lanskap kota mandiri Summarecon Bandung yang asri, jalan lebar bebas hambatan, serta festival kuliner dan hiburan musik setelah garis finish.",
    eventType: "ROAD_RACE",
    status: "published",
    organizerName: "Summarecon Bandung Sports",
    organizerSlug: "summarecon-bandung",
    city: "Bandung",
    province: "Jawa Barat",
    country: "Indonesia",
    venueName: "Summarecon Mall Bandung (Summaba)",
    venueAddress: "Jl. Bulevar Barat No. 1, Cisaranten Kidul, Gedebage, Kota Bandung",
    startsAt: "2026-10-18T05:30:00+07:00",
    flagOffTime: "05:30 WIB",
    bannerUrl: "/images/events/bandung_runfest.jpg",
    logoUrl: "/images/events/bandung_runfest.jpg",
    themeColor: "#10b981",
    accentColor: "#06b6d4",
    certifiedBy: "PASI Jawa Barat",
    elevationGain: "45m (Super Flat Course)",
    waterStationsCount: 8,
    medicalStationsCount: 6,
    categories: [
      {
        id: "cat-sbr-10k",
        code: "10K",
        name: "10K Open & Master",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:15:00",
        minAge: 16,
        price: 375000,
        earlyBirdPrice: 300000,
        quota: 3000,
        registeredCount: 2850,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: "2026-08-15T10:00:00+07:00",
        closesAt: "2026-10-05T23:59:59+07:00",
        includes: [
          "Jersey Running Dry-Fit",
          "Medali Finisher Logam Bergengsi",
          "BIB Number Timing Chip",
          "Refreshment & Buah Segar",
          "Kupon Festival Kuliner Summaba",
        ],
      },
      {
        id: "cat-sbr-5k",
        code: "5K",
        name: "5K Fun Run",
        distance: "5.000 KM",
        distanceKm: 5.0,
        cutoffTime: "01:15:00",
        minAge: 12,
        price: 275000,
        earlyBirdPrice: 225000,
        quota: 4000,
        registeredCount: 3600,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: "2026-08-15T10:00:00+07:00",
        closesAt: "2026-10-05T23:59:59+07:00",
        includes: [
          "Jersey Running Dry-Fit",
          "Medali Finisher Penuh Warna",
          "BIB Number",
          "Refreshment Finisher",
        ],
      },
      {
        id: "cat-sbr-kids",
        code: "KIDS",
        name: "Kids Dash 1.5K",
        distance: "1.500 KM",
        distanceKm: 1.5,
        cutoffTime: "00:45:00",
        minAge: 5,
        price: 180000,
        quota: 1000,
        registeredCount: 890,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: "2026-08-15T10:00:00+07:00",
        closesAt: "2026-10-05T23:59:59+07:00",
        includes: [
          "Jersey Kids Lucu & Nyaman",
          "Medali Finisher Emas Ceria",
          "Paket Snack & Mainan Anak",
        ],
      },
    ],
    routeHighlights: [
      "Boulevard Utama Summarecon Bandung",
      "Danau Gedebage & Ruang Terbuka Hijau",
      "Jalan Aspal Mulus Bebas Kendaraan Motor",
      "Water Mist & DJ Stage di KM 4",
    ],
    racepackInfo: {
      dates: "16 - 17 Oktober 2026",
      venue: "Atrium Summarecon Mall Bandung",
      address: "Gedebage, Kota Bandung",
      hours: "11:00 - 20:00 WIB",
      requirements: [
        "Kartu Identitas KTP/Pelajar/KIA",
        "Bukti E-Ticket QR Code",
      ],
    },
    schedule: [
      { time: "05:00 WIB", activity: "Race Village Buka & Pemanasan Senam Bersama" },
      { time: "05:30 WIB", activity: "Flag-Off 10K Open & Master", category: "10K" },
      { time: "06:00 WIB", activity: "Flag-Off 5K Fun Run", category: "5K" },
      { time: "06:45 WIB", activity: "Flag-Off Kids Dash 1.5K", category: "KIDS" },
      { time: "07:45 WIB", activity: "Cut-Off Time Seluruh Kategori" },
      { time: "08:15 WIB", activity: "Live Music Performance & Doorprize Motor Listrik" },
    ],
    faqs: [
      {
        q: "Apakah peserta anak wajib didampingi?",
        a: "Ya, peserta kategori Kids Dash diperbolehkan didampingi 1 orang tua/wali di lintasan lari tanpa dipungut biaya tambahan.",
      },
      {
        q: "Apakah rute ramah untuk pemula?",
        a: "Sangat ramah! Kontur jalan datar 100% dengan aspal mulus dan lebar sehingga sangat ideal untuk mencapai rekor waktu pribadi (PB).",
      },
    ],
    sponsors: [
      { name: "Summarecon Bandung", role: "Host Venue" },
      { name: "Pocari Sweat", role: "Hydration Partner" },
      { name: "Kahf", role: "Personal Grooming Partner" },
    ],
    layoutConfig: {
      templateId: "template-summarecon",
      heroAlignment: "center",
      heroVariant: "poster",
      themeStyle: "summarecon_neon",
      paperTexture: true,
      showTicker: true,
      tickerText: "NEW BANDUNG · NEW ROADS · NEW POSSIBILITIES · FLAT & FAST PB ROUTE · THE ROAD IS FINALLY YOURS · SUMMARECON FESTIVAL 2026",
      showCountdown: true,
      showCategories: true,
      showRoute: true,
      showRpc: true,
      showSchedule: true,
      showFaq: true,
      showSponsors: true,
      showCustomBlocks: true,
      categorySectionTitle: "RACE CATEGORIES & ENTRY",
      categorySectionSubtitle: "Pilihan kategori 10K, 5K Fun Run, dan Kids Dash. Dilengkapi jersey dry-fit neon eksklusif, medali logam finisher, dan kupon bazar.",
      routeSectionTitle: "LINTASAN & SPESIFIKASI RUTE",
      routeSectionSubtitle: "Lintasan aspal mulus dan lebar 100% datar di boulevard Summarecon Bandung dengan water mist shower KM 4.",
      rpcSectionTitle: "PENGAMBILAN RACEPACK (RPC)",
      rpcSectionSubtitle: "Ambil paket lomba di Atrium Summarecon Mall Bandung dengan membawa QR konfirmasi tiket.",
      scheduleSectionTitle: "RANGKAIAN JADWAL HARI H",
      scheduleSectionSubtitle: "Urutan flag-off pagi, senam bersama, live band performance, dan doorprize utama motor listrik.",
      ctaButtonText: "DAFTAR SEKARANG",
      customBlocks: [
        {
          id: "block-family",
          title: "Kids Dash & Area Ramah Keluarga",
          badge: "FAMILY FRIENDLY",
          style: "card",
          content: "Kategori Kids Dash 1.5K ramah anak dengan pendampingan orang tua gratis. Tersedia playground anak, inflatable castle, dan ice cream station di garis finish.",
        },
        {
          id: "block-foodfest",
          title: "Summaba Food Festival & Live Entertainment",
          badge: "FESTIVAL KULINER",
          style: "banner",
          content: "Setiap nomor BIB peserta mendapatkan voucher belanja senilai Rp 50.000 untuk menikmati aneka kuliner khas Bandung di area Race Village setelah finish.",
        },
      ],
    },
    updatedAt: new Date().toISOString(),
  },
  {
    id: "tokyo-international-marathon-2026",
    slug: "tokyo-international-marathon-2026",
    name: "Tokyo International Marathon 2026",
    tagline: "The Day We Unite: Tokyo World Athletics Elite Major",
    description:
      "Salah satu dari Abbott World Marathon Majors paling prestisius di dunia. Membentang dari Gedung Pemerintah Metropolitan Tokyo di Shinjuku, melintasi Asakusa, Ginza, hingga finish megah di Stasiun Tokyo.",
    eventType: "MARATHON",
    status: "published",
    organizerName: "Tokyo Marathon Foundation",
    organizerSlug: "tokyo-marathon-fdn",
    city: "Tokyo",
    province: "Kanto",
    country: "Jepang",
    venueName: "Tokyo Metropolitan Government Building, Shinjuku",
    venueAddress: "2 Chome-8-1 Nishishinjuku, Shinjuku City, Tokyo 163-8001",
    startsAt: "2026-03-01T09:10:00+09:00",
    flagOffTime: "09:10 JST",
    bannerUrl: "/images/events/tokyo_marathon.jpg",
    logoUrl: "/images/events/tokyo_marathon.jpg",
    themeColor: "#dc2626",
    accentColor: "#1e3a8a",
    certifiedBy: "Abbott World Marathon Majors & World Athletics Platinum",
    elevationGain: "85m (Fast & Net Downhill)",
    waterStationsCount: 22,
    medicalStationsCount: 16,
    categories: [
      {
        id: "cat-tokyo-fm",
        code: "FM42K",
        name: "Marathon (Men, Women, Wheelchair)",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "07:00:00",
        minAge: 19,
        price: 2600000,
        quota: 38000,
        registeredCount: 36500,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: "2026-08-01T10:00:00+09:00",
        closesAt: "2026-09-30T17:00:00+09:00",
        includes: [
          "Official ASICS Race Singlet",
          "Signature Tokyo Finisher Medal",
          "Tokyo Finisher Robe Towel",
          "RFID Timing Tag",
          "Tokyo Metro 24-Hour Pass",
        ],
      },
    ],
    routeHighlights: [
      "Shinjuku Metropolitan Skyscraper District",
      "Kaminarimon Gate & Sensō-ji Temple, Asakusa",
      "Ginza High-End Shopping Boulevard",
      "Tokyo Station Gyoko-dori Grand Finish",
    ],
    racepackInfo: {
      dates: "26 - 28 Februari 2026",
      venue: "Tokyo Big Sight, Odaiba",
      address: "3 Chome-11-1 Ariake, Koto City, Tokyo",
      hours: "10:00 - 20:30 JST",
      requirements: [
        "Paspor Asli",
        "QR Code Registrasi Resmi Tokyo Marathon",
        "Sertifikat Vaksinasi / Medical Clearance",
      ],
    },
    schedule: [
      { time: "09:05 JST", activity: "Flag-Off Wheelchair Marathon" },
      { time: "09:10 JST", activity: "Flag-Off Marathon Wave 1 (Elite & General)", category: "FM42K" },
      { time: "16:10 JST", activity: "Course Cut-off Time (Tokyo Station Closed)" },
    ],
    faqs: [
      {
        q: "Berapa rasio penerimaan undian ballot Tokyo Marathon?",
        a: "Rasio penerimaan umum internasional biasanya berkisar 1:11 hingga 1:13. Pengumuman dikirim via portal resmi pelari.",
      },
    ],
    sponsors: [
      { name: "Tokyo Metro", role: "Presenting Partner" },
      { name: "ASICS", role: "Official Apparel & Shoe" },
      { name: "Seiko", role: "Official Timing Master" },
    ],
    layoutConfig: {
      templateId: "template-tokyo",
      heroAlignment: "center",
      heroVariant: "centered",
      themeStyle: "tokyo_platinum",
      paperTexture: false,
      showTicker: true,
      tickerText: "TOKYO MARATHON 2026 · THE DAY WE UNITE · ABBOTT WORLD MARATHON MAJORS · WORLD ATHLETICS PLATINUM LABEL · SHINJUKU TO TOKYO STATION",
      showCountdown: true,
      showCategories: true,
      showRoute: true,
      showRpc: true,
      showSchedule: true,
      showFaq: true,
      showSponsors: true,
      showCustomBlocks: true,
      categorySectionTitle: "OFFICIAL RACE ENTRY & BALLOT",
      categorySectionSubtitle: "Ajang lari marathon paling prestisius di Asia. Kategori Full Marathon 42.195 km melintasi ikon metropolitan Tokyo.",
      routeSectionTitle: "COURSE MAP & HISTORIC TOKYO LANDMARKS",
      routeSectionSubtitle: "Dari Gedung Pemerintah Metropolitan Tokyo di Shinjuku, melewati Kuil Asakusa Senso-ji, distrik Ginza, hingga Stasiun Tokyo.",
      rpcSectionTitle: "TOKYO MARATHON EXPO (RACEPACK)",
      rpcSectionSubtitle: "Validasi registrasi dan pengambilan race bib resmi di Tokyo Big Sight Odaiba.",
      scheduleSectionTitle: "RACE DAY TIMELINE & CORRALS",
      scheduleSectionSubtitle: "Jadwal wave start ketat demi keselamatan dan kelancaran 38.000 pelari dari 120 negara.",
      ctaButtonText: "DAFTAR UNDIAN BALLOT",
      customBlocks: [
        {
          id: "block-tokyo-pass",
          title: "Tiket Terusan Tokyo Metro Subway 24 Jam Gratis",
          badge: "TRANSPORT PASS",
          style: "card",
          content: "Setiap pelari resmi mendapatkan tiket transportasi Tokyo Metro gratis tanpa batas selama 24 jam untuk memudahkan mobilitas ke Shinjuku dan kembali dari Tokyo Station.",
        },
        {
          id: "block-security",
          title: "Protokol Keamanan & Pemeriksaan Ketat",
          badge: "SAFETY FIRST",
          style: "notice",
          content: "Hanya tas racepack resmi transparan yang diizinkan masuk ke area corral start. Botol kaca dan drone dilarang keras di sepanjang lintasan.",
        },
      ],
    },
    updatedAt: new Date().toISOString(),
  },
  {
    id: "berlin-speed-marathon-2026",
    slug: "berlin-speed-marathon-2026",
    name: "BMW Berlin Speed Marathon 2026",
    tagline: "The World Record Course: Flat, Fast, Legendary",
    description:
      "Lintasan tercepat di muka bumi tempat terciptanya rekor dunia marathon. Melewati landmark bersejarah Jerman, jalanan lurus lebar, dan klimaks legendaris melintasi gerbang Brandenburg Gate menuju finish line.",
    eventType: "MARATHON",
    status: "published",
    organizerName: "SCC EVENTS Berlin",
    organizerSlug: "scc-events",
    city: "Berlin",
    province: "Berlin",
    country: "Jerman",
    venueName: "Straße des 17. Juni, Tiergarten",
    venueAddress: "Straße des 17. Juni, 10785 Berlin, Germany",
    startsAt: "2026-09-27T09:15:00+02:00",
    flagOffTime: "09:15 CEST",
    bannerUrl: "/images/events/hero_marathon.jpg",
    logoUrl: "/images/events/hero_marathon.jpg",
    themeColor: "#0284c7",
    accentColor: "#38bdf8",
    certifiedBy: "Abbott World Marathon Majors & World Athletics Platinum",
    elevationGain: "20m (Flat Record Course)",
    waterStationsCount: 20,
    medicalStationsCount: 15,
    categories: [
      {
        id: "cat-berlin-fm",
        code: "FM42K",
        name: "BMW Berlin Marathon",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "06:15:00",
        minAge: 18,
        price: 3200000,
        quota: 45000,
        registeredCount: 44200,
        mode: "WAR_QUEUE",
        status: "WAR_SOON",
        opensAt: "2026-07-01T10:00:00+02:00",
        closesAt: "2026-08-30T18:00:00+02:00",
        includes: [
          "Adidas Event Running Shirt",
          "Finisher Medal with Brandenburg Gate Motif",
          "Heat Poncho at Finish Line",
          "Official Chip Timing",
        ],
      },
    ],
    routeHighlights: [
      "Start at Straße des 17. Juni (Tiergarten)",
      "Reichstag & Alexanderplatz",
      "Potsdamer Platz & Kurfürstendamm",
      "Crossing the Historic Brandenburg Gate at KM 41.8",
    ],
    racepackInfo: {
      dates: "24 - 26 September 2026",
      venue: "Former Tempelhof Airport (BERLIN VITAL Expo)",
      address: "Platz der Luftbrücke 5, 12101 Berlin",
      hours: "11:00 - 20:00 CEST",
      requirements: ["ID Card / Passport", "Start Card Email with QR Code"],
    },
    schedule: [
      { time: "09:15 CEST", activity: "Start Wave 1 (Elite A & B)" },
      { time: "09:35 CEST", activity: "Start Wave 2" },
      { time: "10:05 CEST", activity: "Start Wave 3" },
      { time: "16:00 CEST", activity: "Official Race Course Close" },
    ],
    faqs: [
      {
        q: "Apakah rute ini benar-benar datar?",
        a: "Ya, Berlin Marathon adalah rute terdatar di antara seluruh World Marathon Majors dengan total elevasi kurang dari 25 meter.",
      },
    ],
    sponsors: [
      { name: "BMW", role: "Title Sponsor" },
      { name: "Adidas", role: "Official Outfitter" },
      { name: "Generali", role: "Community Sponsor" },
    ],
    layoutConfig: {
      templateId: "template-berlin",
      heroAlignment: "left",
      heroVariant: "split",
      themeStyle: "berlin_speed",
      paperTexture: false,
      showTicker: true,
      tickerText: "BMW BERLIN MARATHON 2026 · ROAD TO BERLIN · FASTEST MARATHON IN THE WORLD · FLATTEST WORLD RECORD COURSE · BRANDENBURG GATE",
      showCountdown: true,
      showCategories: true,
      showRoute: true,
      showRpc: true,
      showSchedule: true,
      showFaq: true,
      showSponsors: true,
      showCustomBlocks: true,
      categorySectionTitle: "BMW BERLIN MARATHON CATEGORIES",
      categorySectionSubtitle: "Lintasan legendaris pemecah rekor dunia marathon dunia. Datar, cepat, dan spektakuler.",
      routeSectionTitle: "FASTEST COURSE PROFILE & ELEVATION",
      routeSectionSubtitle: "Elevasi nyaris nol dengan aspal berkualitas tinggi, garis biru terpendek (tangent line), dan finish spektakuler di Gerbang Brandenburg.",
      rpcSectionTitle: "BERLIN VITAL EXPO (RACEPACK)",
      rpcSectionSubtitle: "Penukaran nomor dada BIB di Bandara Bersejarah Tempelhof Hanggar 5-7.",
      scheduleSectionTitle: "START WAVES & RACE TIMELINE",
      scheduleSectionSubtitle: "Gelombang start elite, pelari roda tiga/handbike, dan wave pelari umum.",
      ctaButtonText: "DAFTAR ANTRIAN WAR",
      customBlocks: [
        {
          id: "block-world-record",
          title: "Lintasan Rekor Dunia: Elevasi Hanya 20 Meter",
          badge: "WORLD RECORD",
          style: "banner",
          content: "13 rekor dunia marathon putra dan putri tercipta di rute ini. Jika Anda mengincar catatan waktu Personal Best (PB) atau Boston Qualifier, Berlin adalah tempatnya.",
        },
        {
          id: "block-brandenburg",
          title: "Klimaks KM 41.8: Gerbang Brandenburg yang Magis",
          badge: "HISTORIC FINISH",
          style: "card",
          content: "Rasakan euforia melintasi gerbang Brandenburg Gate dengan sorakan lebih dari 1 juta penonton di jalanan kota Berlin sebelum menyentuh garis finish.",
        },
      ],
    },
    updatedAt: new Date().toISOString(),
  },
  {
    id: "sydney-harbour-marathon-2026",
    slug: "sydney-harbour-marathon-2026",
    name: "TCS Sydney Harbour Marathon 2026",
    tagline: "Run Across the Iconic Harbour Bridge to the Opera House",
    description:
      "Ajang marathon paling ikonik di Australia dan kandidat resmi ketujuh Abbott World Marathon Majors. Ribuan pelari melintasi jembatan legendaris Sydney Harbour Bridge dengan pemandangan cakrawala kota dan hembusan angin samudra menuju finish spektakuler di Sydney Opera House Forecourt.",
    eventType: "MARATHON",
    status: "published",
    organizerName: "Pont3 / Athletics Australia",
    organizerSlug: "sydney-marathon-org",
    city: "Sydney",
    province: "New South Wales",
    country: "Australia",
    venueName: "Bradfield Park, Milsons Point (Harbour Bridge)",
    venueAddress: "Alfred St S, Milsons Point NSW 2061, Australia",
    startsAt: "2026-08-30T07:05:00+10:00",
    flagOffTime: "07:05 AEST",
    bannerUrl: "/images/events/hero_marathon.jpg",
    logoUrl: "/images/events/hero_marathon.jpg",
    themeColor: "#0284c7",
    accentColor: "#0d9488",
    certifiedBy: "World Athletics Platinum & Abbott WMM Candidate",
    elevationGain: "165m (Rolling Bridge & Coastal Park)",
    waterStationsCount: 18,
    medicalStationsCount: 12,
    categories: [
      {
        id: "cat-syd-fm",
        code: "FM42K",
        name: "TCS Sydney Marathon",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "07:00:00",
        minAge: 18,
        price: 2800000,
        quota: 40000,
        registeredCount: 38200,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: "2026-05-01T09:00:00+10:00",
        closesAt: "2026-07-15T23:59:59+10:00",
        includes: [
          "Official ASICS Technical Singlet",
          "Opera House Finisher Medal",
          "Finisher Poncho",
          "RFID Timing Bib",
          "Sydney Public Transport Day Pass",
        ],
      },
      {
        id: "cat-syd-hm",
        code: "HM21K",
        name: "Sydney Half Marathon",
        distance: "21.097 KM",
        distanceKm: 21.097,
        cutoffTime: "03:30:00",
        minAge: 16,
        price: 1850000,
        quota: 15000,
        registeredCount: 14100,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: "2026-05-01T09:00:00+10:00",
        closesAt: "2026-07-30T23:59:59+10:00",
        includes: [
          "Official ASICS Event Tee",
          "Finisher Medal",
          "RFID Timing Bib",
        ],
      },
      {
        id: "cat-syd-mini",
        code: "MINI10K",
        name: "TCS Sydney Mini Marathon 10K",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:00:00",
        minAge: 12,
        price: 950000,
        quota: 10000,
        registeredCount: 8900,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: "2026-05-01T09:00:00+10:00",
        closesAt: "2026-08-10T23:59:59+10:00",
        includes: [
          "Running Shirt",
          "Medali Finisher Sydney",
          "BIB Number",
        ],
      },
    ],
    routeHighlights: [
      "Crossing the Iconic Sydney Harbour Bridge Deck",
      "Royal Botanic Garden & Mrs Macquarie's Chair",
      "Centennial Park Rolling Boulevard",
      "Grand Finish at the Forecourt of the Sydney Opera House",
    ],
    racepackInfo: {
      dates: "27 - 29 Agustus 2026",
      venue: "International Convention Centre (ICC) Sydney, Darling Harbour",
      address: "14 Darling Dr, Sydney NSW 2000",
      hours: "09:00 - 19:00 AEST",
      requirements: [
        "Passport / Driver Licence",
        "Official Confirmation QR Code",
      ],
    },
    schedule: [
      { time: "06:00 AEST", activity: "Assembly Areas Open (Purple, Green, Orange)" },
      { time: "07:05 AEST", activity: "Flag-Off Wheelchair & Elite Marathon", category: "FM42K" },
      { time: "07:20 AEST", activity: "Flag-Off Wave 1 Full Marathon" },
      { time: "08:30 AEST", activity: "Flag-Off Half Marathon & Mini Marathon", category: "HM21K" },
      { time: "14:30 AEST", activity: "Opera House Finish Line Close" },
    ],
    faqs: [
      {
        q: "Bagaimana pembagian zona start di Sydney Marathon?",
        a: "Zona start dibagi berdasarkan wave warna di Bradfield Park: Purple (sub-3:15), Green (sub-3:45), Orange (sub-4:30), dan General Wave.",
      },
    ],
    sponsors: [
      { name: "TCS", role: "Title Sponsor" },
      { name: "ASICS", role: "Official Outfitter" },
      { name: "New South Wales Government", role: "Destination NSW Host" },
    ],
    layoutConfig: {
      templateId: "template-sydney",
      heroAlignment: "left",
      heroVariant: "split",
      themeStyle: "berlin_speed",
      paperTexture: false,
      showTicker: true,
      tickerText: "TCS SYDNEY MARATHON · AUSTRALIA'S ABBOTT WORLD MARATHON MAJOR · RUN ACROSS HARBOUR BRIDGE TO THE OPERA HOUSE",
      showCountdown: true,
      showCategories: true,
      showRoute: true,
      showRpc: true,
      showSchedule: true,
      showFaq: true,
      showSponsors: true,
      showCustomBlocks: true,
      categorySectionTitle: "OFFICIAL RACES & ENTRY",
      categorySectionSubtitle: "Pilihan kategori lomba resmi. Lintasi Harbour Bridge bebas kendaraan menuju pelataran Sydney Opera House.",
      routeSectionTitle: "COASTAL & HARBOUR BRIDGE PROFILE",
      routeSectionSubtitle: "Membentang dari Milsons Point melintasi jembatan ikonik, Centennial Park, hingga Opera House Forecourt.",
      rpcSectionTitle: "SYDNEY RUNNING SHOW (RACEPACK)",
      rpcSectionSubtitle: "Ambil race bib resmi Anda di ICC Sydney Darling Harbour.",
      scheduleSectionTitle: "RACE DAY SCHEDULE & WAVE START",
      scheduleSectionSubtitle: "Jadwal corral wave Purple, Green, Orange dan penutupan rute.",
      ctaButtonText: "DAFTAR SEKARANG",
      customBlocks: [
        {
          id: "block-wave-corral",
          title: "Pembagian Assembly Area Wave Warna",
          badge: "START ZONES",
          style: "card",
          content: "Peserta wajib memasuki zona start sesuai warna BIB: Purple (Gate A), Green (Gate B), dan Orange (Gate C) di Bradfield Park Milsons Point.",
        },
        {
          id: "block-ferry-transit",
          title: "Transportasi Kereta & Feri Gratis Menuju Start",
          badge: "FREE TRANSIT",
          style: "banner",
          content: "Semua peserta resmi mendapatkan perjalanan gratis di jaringan kereta dan feri Sydney Trains pada Hari H dengan menunjukkan BIB resmi.",
        },
      ],
    },
    updatedAt: new Date().toISOString(),
  },
  {
    id: "bangkok-midnight-marathon-2026",
    slug: "bangkok-midnight-marathon-2026",
    name: "Bangkok Midnight Marathon 2026 (BDMS)",
    tagline: "Run Under the Neon & Gold of the Grand Palace and Rama VIII Bridge",
    description:
      "Marathon malam paling prestisius di Asia Tenggara dengan start tengah malam di depan Grand Palace Sanam Chai Road, melintasi gemerlap Jembatan Kabel Rama VIII dan jalan layang elevated highway Borommaratchachonnani yang sejuk dan bebas polusi.",
    eventType: "MARATHON",
    status: "published",
    organizerName: "National Jogging Association of Thailand (NJAT)",
    organizerSlug: "njat-bangkok",
    city: "Bangkok",
    province: "Bangkok Metropolis",
    country: "Thailand",
    venueName: "Sanam Chai Road (Grand Palace / Wat Phra Kaew)",
    venueAddress: "Sanam Chai Rd, Phra Borom Maha Ratchawang, Phra Nakhon, Bangkok 10200, Thailand",
    startsAt: "2026-11-22T00:30:00+07:00",
    flagOffTime: "00:30 ICT",
    bannerUrl: "/images/events/bangkok_marathon.jpg",
    logoUrl: "/images/events/bangkok_marathon.jpg",
    themeColor: "#f45227",
    accentColor: "#ed9227",
    certifiedBy: "World Athletics & AIMS Label Road Race",
    elevationGain: "45m (Rama VIII Bridge & Elevated Skyway)",
    waterStationsCount: 20,
    medicalStationsCount: 14,
    categories: [
      {
        id: "cat-bkk-fm",
        code: "FM42K",
        name: "Full Marathon Midnight",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "06:00:00",
        minAge: 18,
        price: 1450000,
        quota: 5000,
        registeredCount: 4200,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: "2026-05-01T00:00:00+07:00",
        closesAt: "2026-10-15T23:59:59+07:00",
        includes: [
          "Official BDMS Running Singlet",
          "Finisher Tee Lengan Panjang",
          "Medali Cor Emas Grand Palace",
          "BIB Timing Chip RFID",
          "Paket Refreshment Malam & Buah Tropis",
        ],
      },
      {
        id: "cat-bkk-hm",
        code: "HM21K",
        name: "Half Marathon Midnight",
        distance: "21.097 KM",
        distanceKm: 21.097,
        cutoffTime: "03:30:00",
        minAge: 16,
        price: 1150000,
        quota: 8000,
        registeredCount: 7100,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: "2026-05-01T00:00:00+07:00",
        closesAt: "2026-10-15T23:59:59+07:00",
        includes: [
          "Official BDMS Running Singlet",
          "Medali Finisher 21K",
          "BIB Timing Chip RFID",
          "Refreshment",
        ],
      },
      {
        id: "cat-bkk-10k",
        code: "10K",
        name: "Mini Marathon 10K",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:00:00",
        minAge: 14,
        price: 850000,
        quota: 10000,
        registeredCount: 8800,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: "2026-05-01T00:00:00+07:00",
        closesAt: "2026-10-30T23:59:59+07:00",
        includes: [
          "Official Event Running Tee",
          "Medali Finisher 10K",
          "BIB RFID",
        ],
      },
      {
        id: "cat-bkk-5k",
        code: "5K",
        name: "Micro Marathon 5K Fun Run",
        distance: "5.000 KM",
        distanceKm: 5.0,
        cutoffTime: "01:00:00",
        minAge: 10,
        price: 600000,
        quota: 5000,
        registeredCount: 4300,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: "2026-05-01T00:00:00+07:00",
        closesAt: "2026-10-30T23:59:59+07:00",
        includes: [
          "Official Running Tee",
          "Medali Finisher 5K",
          "BIB Number",
        ],
      },
    ],
    routeHighlights: [
      "Start Garis Depan Grand Palace & Wat Phra Kaew Sanam Chai Road",
      "Melintasi Kemegahan Jembatan Kabel Rama VIII di Atas Sungai Chao Phraya",
      "Jalur Melayang Bebas Hambatan Borommaratchachonnani Elevated Road",
      "Finish Subuh Gemerlap Menikmati Fajar Pertama Kota Bangkok",
    ],
    racepackInfo: {
      dates: "19 - 21 November 2026",
      venue: "Royal Paragon Hall, Siam Paragon Bangkok",
      address: "991 Rama I Rd, Pathum Wan, Bangkok 10330, Thailand",
      hours: "10:00 - 20:00 ICT",
      requirements: [
        "Paspor Resmi / Thai National ID",
        "QR Code Konfirmasi Pendaftaran",
      ],
    },
    schedule: [
      { time: "23:00 ICT", activity: "Race Village Sanam Chai Assembly & Drop Bag Open" },
      { time: "00:30 ICT", activity: "Flag-Off Full Marathon 42.195 KM (COT 6 Jam)", category: "FM42K" },
      { time: "03:00 ICT", activity: "Flag-Off Half Marathon 21.097 KM (COT 3.5 Jam)", category: "HM21K" },
      { time: "04:30 ICT", activity: "Flag-Off Mini Marathon 10 KM (COT 2 Jam)", category: "10K" },
      { time: "05:00 ICT", activity: "Flag-Off Micro Marathon 5 KM", category: "5K" },
      { time: "06:30 ICT", activity: "Grand Palace Sunrise Finisher Ceremony & Closing" },
    ],
    faqs: [
      {
        q: "Mengapa flag-off Full Marathon dilakukan pukul 00:30 tengah malam?",
        a: "Jadwal tengah malam dirancang khusus untuk menghindari terik matahari tropis Bangkok, menjamin udara sejuk, dan memberikan pengalaman spektakuler berlari di bawah gemerlap lampu Jembatan Rama VIII.",
      },
      {
        q: "Apakah seluruh rute steril dari kendaraan bermotor?",
        a: "Ya, seluruh rute jalan raya dan jalan layang Borommaratchachonnani ditutup total oleh Polisi Lalu Lintas Kerajaan Thailand demi keamanan maksimal pelari.",
      },
    ],
    sponsors: [
      { name: "BDMS Bangkok Dusit Medical Services", role: "Title Sponsor" },
      { name: "Mizuno", role: "Official Apparel & Footwear" },
      { name: "Singha Corporation", role: "Official Beverage" },
    ],
    layoutConfig: {
      templateId: "template-bangkok",
      heroAlignment: "center",
      heroVariant: "centered",
      themeStyle: "custom",
      customThemePrimary: "#f45227",
      customThemeAccent: "#ed9227",
      paperTexture: false,
      showTicker: true,
      tickerText: "THE 37TH BANGKOK MARATHON · SANAM CHAI GRAND PALACE · RAMA VIII SUSPENSION BRIDGE · MIDNIGHT START 00:30 ICT",
      showCountdown: true,
      showCategories: true,
      showRoute: true,
      showRpc: true,
      showSchedule: true,
      showFaq: true,
      showSponsors: true,
      showCustomBlocks: true,
      categorySectionTitle: "KATEGORI TIKET LOMBA BANGKOK MARATHON",
      categorySectionSubtitle: "Pilih jarak tantangan lari Anda. Dapatkan nomor BIB resmi, medali cor berlapis emas, dan pengalaman lari malam paling berkesan.",
      routeSectionTitle: "SPESIFIKASI RUTE MALAM & JEMBATAN RAMA VIII",
      routeSectionSubtitle: "Elevasi rata dan steril melintasi Jembatan Kabel Rama VIII dan jalan layang elevated highway.",
      rpcSectionTitle: "PENGAMBILAN RACEPACK (RPC EXPO)",
      rpcSectionSubtitle: "Pengambilan paket lomba resmi di Royal Paragon Hall Siam Paragon.",
      scheduleSectionTitle: "JADWAL FLAG-OFF TENGAH MALAM",
      scheduleSectionSubtitle: "Urutan start kategori mulai pukul 00:30 ICT hingga matahari terbit di Grand Palace.",
      ctaButtonText: "DAFTAR SEKARANG",
      customBlocks: [
        {
          id: "block-bkk-night",
          title: "Pengalaman Unik Lari Tengah Malam (Midnight Race)",
          badge: "MIDNIGHT SPECIAL",
          style: "card",
          content: "Bangkok Marathon digelar tengah malam mulai 00:30 ICT untuk memberikan suhu udara tropis yang optimal (24°C - 26°C), kelembapan nyaman, dan pemandangan lampu-lampu megah kuil bersejarah serta Jembatan Rama VIII.",
        },
        {
          id: "block-bkk-safety",
          title: "Protokol Medis BDMS Kelas Dunia",
          badge: "MEDIS & KEAMANAN",
          style: "banner",
          content: "Didukung penuh oleh BDMS (Bangkok Dusit Medical Services) dengan 20 stasiun hidrasi dingin, ambulans mobile setiap 2 km, dan tim dokter spesialis olahraga di sepanjang lintasan.",
        },
      ],
      bangkokConfig: {
        runnerPhotoUrl: "/images/events/bangkok_runner_action.jpg",
        midnightCallout: "00:30 ICT · DEPAN GRAND PALACE SANAM CHAI",
        topRibbonText: "THE 37TH BANGKOK MARATHON · SANAM CHAI GRAND PALACE · RAMA VIII BRIDGE · MIDNIGHT START 00:30 ICT",
        skewBadgeText: "THE 37TH BANGKOK MARATHON (BDMS)",
      },
    },
    updatedAt: new Date().toISOString(),
  },
  {
    id: "ironman-703-triathlon-2026",
    slug: "ironman-703-triathlon-2026",
    name: "IRONMAN 70.3 Championship Triathlon 2026",
    tagline: "1.9KM Swim, 90KM Bike, 21.1KM Run",
    description:
      "Ajang ketahanan fisik puncak dunia World Triathlon Series. Uji batas kemampuan Anda melalui renang perairan terbuka di teluk tropis berair tenang, balap sepeda time-trial 90 km di jalanan aspal pesisir, dan lari setengah marathon 21.1 km berujung di karpet merah panggung penobatan atlet legendaris.",
    eventType: "TRIATHLON",
    status: "published",
    organizerName: "The IRONMAN Group & World Triathlon Corporation",
    organizerSlug: "ironman-global",
    city: "Langkawi",
    province: "Kedah",
    country: "Malaysia",
    venueName: "Pelangi Beach Resort & Spa, Pantai Cenang",
    venueAddress: "Pantai Cenang, 07000 Langkawi, Kedah, Malaysia",
    startsAt: "2026-10-18T06:30:00+08:00",
    flagOffTime: "06:30 MYT",
    bannerUrl: "/images/events/ironman_triathlon.jpg",
    logoUrl: "/images/events/ironman_triathlon.jpg",
    themeColor: "#e10600",
    accentColor: "#171717",
    certifiedBy: "World Triathlon & IRONMAN Pro Series Official",
    elevationGain: "480m (Rolling Coastal Highway & Jungle Foothills)",
    waterStationsCount: 24,
    medicalStationsCount: 16,
    categories: [
      {
        id: "cat-im-703-indiv",
        code: "IM 70.3",
        name: "IRONMAN 70.3 Individual",
        distance: "113.0 KM (1.9K Swim / 90K Bike / 21.1K Run)",
        distanceKm: 113.0,
        cutoffTime: "08:30:00",
        minAge: 18,
        price: 6250000,
        quota: 2500,
        registeredCount: 2150,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: "2026-03-01T09:00:00+08:00",
        closesAt: "2026-09-15T23:59:59+08:00",
        includes: [
          "Official IRONMAN Finisher Polo & Backpack",
          "Finisher Medal Logam Cor M-Dot Berat",
          "Topi Renang Silikon Resmi & Nomor Race Bib",
          "Gelang Timing Transponder T1/T2",
          "Akses Perjamuan Makan Malam Pemenang (Awards Banquet)",
        ],
      },
      {
        id: "cat-im-703-relay",
        code: "RELAY 70.3",
        name: "IRONMAN 70.3 Team Relay (2-3 Atlet)",
        distance: "113.0 KM (Tim Perenang, Pesepeda, Pelari)",
        distanceKm: 113.0,
        cutoffTime: "08:30:00",
        minAge: 18,
        price: 8500000,
        quota: 300,
        registeredCount: 260,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: "2026-03-01T09:00:00+08:00",
        closesAt: "2026-09-15T23:59:59+08:00",
        includes: [
          "3 Paket Finisher Gear & Medali Tim",
          "Transponder Relay Timing Chip",
          "Akses Athlete Village & Recovery Zone",
        ],
      },
      {
        id: "cat-im-1406-full",
        code: "FULL 140.6",
        name: "IRONMAN Full Distance 140.6",
        distance: "226.0 KM (3.8K Swim / 180.2K Bike / 42.2K Run)",
        distanceKm: 226.0,
        cutoffTime: "17:00:00",
        minAge: 18,
        price: 11500000,
        quota: 1200,
        registeredCount: 980,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: "2026-03-01T09:00:00+08:00",
        closesAt: "2026-08-30T23:59:59+08:00",
        includes: [
          "Finisher Jacket Eksklusif IRONMAN Full 140.6",
          "Medali Cor Emas M-Dot Finisher",
          "Finisher Towel & Hat",
          "Akses Welcome Dinner & Slot Kona Rolldown",
        ],
      },
    ],
    routeHighlights: [
      "Renang 1.9K di Perairan Tenang Teluk Pesisir Samudra Pantai Cenang",
      "Sepeda 90K Melintasi Sirkuit Aspal Mulus Pantai Pasir Tengkorak & Kaki Gunung Raya",
      "Lari 21.1K Sepanjang Boulevard Pesisir Pantai Cenang yang Dipadati Penonton",
      "Panggung Finish Berkarpet Merah dengan Sorotan Cahaya 'YOU ARE AN IRONMAN!'",
    ],
    racepackInfo: {
      dates: "15 - 17 Oktober 2026",
      venue: "IRONMAN Village, Pelangi Beach Resort Langkawi",
      address: "Pantai Cenang, Mukim Kedawang, Langkawi 07000, Malaysia",
      hours: "09:00 - 18:00 MYT",
      requirements: [
        "Paspor Asli / National ID",
        "Lisensi Triathlon Nasional / One-Day Race Pass",
        "QR Code Registrasi & Surat Pernyataan Kesehatan (Waiver)",
      ],
    },
    schedule: [
      { time: "05:00 MYT", activity: "Transition Zone (T1/T2) Opens & Tire Pumping" },
      { time: "06:15 MYT", activity: "Transition Area Closes & Swim Assembly at Beach" },
      { time: "06:30 MYT", activity: "Pro Men & Pro Women Rolling Start" },
      { time: "06:40 MYT", activity: "Age Group Rolling Wave Swim Start", category: "IM 70.3" },
      { time: "08:15 MYT", activity: "Swim Cut-Off (1h 10m from last swimmer)" },
      { time: "13:30 MYT", activity: "Intermediate Bike Cut-Off (5h 30m total elapsed)" },
      { time: "16:45 MYT", activity: "Official Race Finish Line Cut-Off (8h 30m total)", category: "IM 70.3" },
      { time: "18:00 MYT", activity: "VinFast IRONMAN World Championship Kona Slot Allocation Ceremony" },
    ],
    faqs: [
      {
        q: "Bagaimana aturan penggunaan wetsuit untuk segmen renang?",
        a: "Sesuai regulasi resmi World Triathlon & IRONMAN, jika suhu air berada di bawah 24.5°C maka wetsuit diperbolehkan (wetsuit legal). Jika suhu melebihi 28.8°C, penggunaan wetsuit dilarang demi keselamatan atlet dari sengatan panas.",
      },
      {
        q: "Apakah balap sepeda memperbolehkan sistem drafting?",
        a: "Tidak. IRONMAN adalah balapan non-drafting. Peserta wajib menjaga jarak minimum 12 meter di belakang pesepeda lain, kecuali saat melakukan manuver menyalip dalam batas waktu maksimal 25 detik.",
      },
      {
        q: "Berapa banyak slot kualifikasi World Championship Kona yang tersedia?",
        a: "Tersedia 45 slot kelompok umur (Age-Group Qualifying Slots) menuju VinFast IRONMAN World Championship di Kailua-Kona, Hawaii.",
      },
    ],
    sponsors: [
      { name: "VinFast", role: "Global Title Partner" },
      { name: "HOKA", role: "Official Running Shoe" },
      { name: "ROKA", role: "Official Swimwear & Eyewear" },
      { name: "FulGaz", role: "Official Virtual Cycling Partner" },
    ],
    layoutConfig: {
      templateId: "template-ironman",
      heroAlignment: "left",
      heroVariant: "poster",
      themeStyle: "custom",
      customThemePrimary: "#e10600",
      customThemeAccent: "#171717",
      paperTexture: false,
      showTicker: true,
      tickerText: "VINFAST IRONMAN WORLD CHAMPIONSHIP QUALIFIER · 45 AGE GROUP SLOTS TO KONA · YOU ARE AN IRONMAN",
      showCountdown: true,
      showCategories: true,
      showRoute: true,
      showRpc: true,
      showSchedule: true,
      showFaq: true,
      showSponsors: true,
      showCustomBlocks: true,
      categorySectionTitle: "KATEGORI TRIATHLON & JARAK RESMI",
      categorySectionSubtitle: "Pilih disiplin tantangan Anda: Individual 70.3, Tim Relay estafet beregu, atau jarak legendaris Full Distance 140.6.",
      routeSectionTitle: "PROFIL MULTI-DISIPLIN: SWIM, BIKE, RUN",
      routeSectionSubtitle: "Spesifikasi teknis lintasan renang samudra, jalur sepeda jalan raya aspal steril, dan sirkuit lari berlatar pesisir tropis.",
      rpcSectionTitle: "CHECK-IN ATLET & BIKE RACKING (RPC)",
      rpcSectionSubtitle: "Pemeriksaan kelengkapan teknis sepeda, penyerahan transition bag (T1/T2), dan pengambilan bib nomor atlet.",
      scheduleSectionTitle: "RACE DAY TIMELINE & CUT-OFF TIMES BERTINGKAT",
      scheduleSectionSubtitle: "Jadwal flag-off gelombang bergulir, batas waktu cut-off disiplin, dan seremoni perebutan slot Kona Hawaii.",
      ctaButtonText: "DAFTAR SEKARANG",
      customBlocks: [
        {
          id: "block-im-slots",
          title: "Alokasi 45 Slot Menuju VinFast IRONMAN World Championship Kona",
          badge: "KONA QUALIFIER",
          style: "banner",
          content: "Perlombaan ini menyediakan 45 slot kualifikasi kelompok umur resmi menuju ajang triathlon paling bergengsi di dunia: VinFast IRONMAN World Championship di Kailua-Kona, Hawaii. Seremoni rolldown slot diselenggarakan pada sore hari setelah lomba.",
        },
        {
          id: "block-im-rules",
          title: "Aturan Non-Drafting Sepeda & Zona Penalti (Penalty Tent)",
          badge: "ATURAN LOMBA",
          style: "notice",
          content: "Drafting di lintasan sepeda dilarang keras. Jarak minimum antar sepeda adalah 12 meter dari ban depan ke ban depan. Pelanggaran kartu biru akan dikenakan penalti waktu 5 menit yang wajib dijalani di tenda penalti terdekat.",
        },
      ],
      ironmanConfig: {
        tagBadge: "FLEX90 ELIGIBLE",
        heroCtaPrimaryText: "VIEW ENTRY OPTIONS",
        heroCtaPrimaryLink: "#im-tiers",
        heroCtaSecondaryText: "COURSE OVERVIEW",
        heroCtaSecondaryLink: "#im-course",
        metrics: [
          { id: "swim", label: "Swim", value: "Ocean (1.9K)", iconType: "preset", presetIcon: "swim" },
          { id: "bike", label: "Bike", value: "Hilly (90K)", iconType: "preset", presetIcon: "bike" },
          { id: "run", label: "Run", value: "Flat (21.1K)", iconType: "preset", presetIcon: "run" },
          { id: "airHigh", label: "High Air Temp", value: "86 °F / 30 °C", iconType: "preset", presetIcon: "sun" },
          { id: "airLow", label: "Low Air Temp", value: "75 °F / 24 °C", iconType: "preset", presetIcon: "snowflake" },
          { id: "waterTemp", label: "Avg. Water Temp", value: "84 °F / 29 °C", iconType: "preset", presetIcon: "water" },
        ],
        swimMetric: "Ocean (1.9K)",
        bikeMetric: "Hilly (90K)",
        runMetric: "Flat (21.1K)",
        airTempMetric: "86 °F / 30 °C",
        lowAirTempMetric: "75 °F / 24 °C",
        waterTempMetric: "84 °F / 29 °C",
        konaSlots: "45 SLOTS",
        splendorTitle: "DIVE INTO THE TROPICAL SPLENDOR OF LANGKAWI",
        splendorDescription: "Rasakan sambutan hangat khas kepulauan Langkawi di sepanjang Pantai Cenang, kekayaan geopark tertua di Asia Tenggara, dan lintasan aspal mulus berlatar bukit karst serta perairan tenang Laut Andaman. Panggung finish legendaris menanti Anda di karpet merah dengan sorotan ribuan penonton.",
        youtubeUrl: "https://www.youtube.com/watch?v=17mCq3YV6c8",
        videoThumbnailUrl: "/images/events/ironman_triathlon.jpg",
        videoBadgeText: "Official Race Rewind & Highlights",
        seals: [
          { id: "seal-1", badgeTag: "QUALIFYING", title: "VinFast World Championship", subtitle: "45 Age-Group Slots to Kona", colorTheme: "blue" },
          { id: "seal-2", badgeTag: "ATHLETES' CHOICE", title: "Athletes' Choice Award", subtitle: "Top-Rated Global Race Experience", colorTheme: "gold" },
          { id: "seal-3", badgeTag: "FLEX90", title: "Flex90 Registration", subtitle: "Free deferral & transfer options within 90 days", colorTheme: "red" },
        ],
        whySectionTagline: "RACE IN PARADISE",
        whySectionTitle: "WHY DO IRONMAN 70.3 LANGKAWI",
        whySectionSubtitle: "Pengalaman triathlon kelas dunia di salah satu destinasi kepulauan tropis terindah di Asia Tenggara.",
        whyBgColor: "#c35607",
        whyCards: [
          { id: "why-1", iconEmoji: "🏖️", title: "Cenang Beach Finish", description: "Finish di pasir putih Pantai Cenang di bawah sorak ribuan penonton dengan panorama magis matahari terbenam Samudra Hindia." },
          { id: "why-2", iconEmoji: "❄️", title: "Comfort Meets Performance", description: "Satu-satunya zona transisi sepeda indoor berpendingin udara (AC) di sirkuit World Series (MIEC Hall)." },
          { id: "why-3", iconEmoji: "🌴", title: "Langkawi Racecation", description: "Berlomba sekaligus berlibur di pulau bebas bea dengan status UNESCO Global Geopark dan resor bintang lima dunia." },
          { id: "why-4", iconEmoji: "🤝", title: "Split The Distance", description: "Kategori Team Relay memungkinkan skuad 2 atau 3 atlet menuntaskan 1.9 km renang, 90 km sepeda, dan 21.1 km lari bersama." },
          { id: "why-5", iconEmoji: "💖", title: "Heartfelt Island Hospitality", description: "Dukungan tulus lebih dari 2.000 sukarelawan lokal di setiap pos hidrasi dan lintasan perlombaan." },
          { id: "why-6", iconEmoji: "⛰️", title: "Diverse Coastal Landscapes", description: "Lintasan bervariasi dari teluk tenang Pantai Kok, jalan pesisir Teluk Yu, kaki hutan Gunung Raya, hingga runway bandara." },
        ],
        showStickyMobileCta: true,
        stickyMobileTitle: "Lock in your spot",
        stickyMobileSubtitle: "Early bird tier 1 open",
        stickyMobileBtnText: "View options",
        stickyMobileBtnLink: "#im-tiers",
      },
      queueConfig: {
        enabled: true,
        mode: "auto",
        trafficThreshold: 50,
        activeWaitingCount: 0,
        releaseRatePerMinute: 60,
      },
    },
    updatedAt: new Date().toISOString(),
  },
  {
    id: "boston-marathon-2026",
    slug: "boston-marathon-2026",
    name: "130th Boston Marathon presented by Bank of America",
    tagline: "To Finish Here Is Everything · Hopkinton to Copley Square",
    description:
      "Ajang perlombaan lari marathon tahunan tertua dan paling legendaris di dunia yang diselenggarakan oleh Boston Athletic Association (B.A.A.) sejak 1897. Menghadirkan lintasan titik-ke-titik bersejarah dari Hopkinton melintasi Wellesley Scream Tunnel, Heartbreak Hill, hingga garis finish ikonik di Boylston Street, Copley Square, Boston.",
    eventType: "MARATHON",
    status: "published",
    organizerName: "Boston Athletic Association (B.A.A.)",
    organizerSlug: "baa",
    city: "Boston, Massachusetts",
    province: "Massachusetts",
    country: "United States",
    venueName: "Hopkinton Main St to Boylston St Copley Square",
    venueAddress: "Hopkinton Town Common (Start) to Copley Square, Boston, MA 02116",
    startsAt: "2026-04-20T09:00:00-04:00",
    flagOffTime: "09:02 EDT (Wave 1)",
    bannerUrl: "/images/events/boston_marathon.jpg",
    logoUrl: "/images/events/boston_baa_logo.png",
    themeColor: "#002244",
    accentColor: "#fdda24",
    certifiedBy: "World Athletics Platinum Label & Abbott World Marathon Majors",
    elevationGain: "-135m (Net Downhill Point-to-Point, 4 Newton Hills Peak at Mile 20.5)",
    waterStationsCount: 26,
    medicalStationsCount: 26,
    categories: [
      {
        id: "cat-baa-fm-qualifier",
        code: "BAA 42K",
        name: "Boston Marathon Qualifier (Time Standard)",
        distance: "42.195 KM (Hopkinton to Boston)",
        distanceKm: 42.195,
        cutoffTime: "06:00:00",
        minAge: 18,
        price: 3850000,
        quota: 30000,
        registeredCount: 0,
        mode: "PRIORITY_ACCESS",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: [
          "Official adidas Boston Marathon Celebration Jacket Voucher",
          "Iconic Unicorn Finisher Medal",
          "Athletes Village Hopkinton Access",
          "Boylston St Recovery Zone",
        ],
      },
      {
        id: "cat-baa-para",
        code: "PARA",
        name: "Para Athletics Division & Wheelchair",
        distance: "42.195 KM (Handcycle & Wheelchair Division)",
        distanceKm: 42.195,
        cutoffTime: "05:00:00",
        minAge: 18,
        price: 1950000,
        quota: 500,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: [
          "Official adidas Gear Pack",
          "Unicorn Finisher Medal",
          "Dedicated Wave Start",
        ],
      },
      {
        id: "cat-baa-5k",
        code: "BAA 5K",
        name: "B.A.A. 5K presented by Point32Health",
        distance: "5.000 KM (Boston Common & Back Bay)",
        distanceKm: 5.0,
        cutoffTime: "01:15:00",
        minAge: 12,
        price: 950000,
        quota: 10000,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: [
          "Official B.A.A. 5K Tech Shirt",
          "Medali Finisher 5K",
          "Timing Tag",
        ],
      },
    ],
    routeHighlights: [
      "Hopkinton Town Common: Titik kumpul dan garis start bersejarah 30.000 pelari (Mile 0)",
      "Framingham & Natick: Sorak sorai ribuan warga New England dan jalur kereta (Mile 6.7 - 10)",
      "Wellesley College Scream Tunnel: Koridor sorak mahasiswi paling bising di dunia maraton (Mile 13.1)",
      "Heartbreak Hill (Mile 20.5): Tanjakan legendaris penentu batas mental dan fisik setelah Newton Fire Station",
      "Right on Hereford, Left on Boylston (Mile 25.8): Tikungan keramat penanda 600 meter terakhir menuju kemuliaan",
      "Finish Line Copley Square (Mile 26.2): Garis finish bercat kuning-biru ikonik di depan Boston Public Library",
    ],
    racepackInfo: {
      dates: "17-19 April 2026 (Jumat - Minggu Race Week)",
      venue: "John B. Hynes Veterans Memorial Convention Center",
      address: "900 Boylston St, Boston, MA 02115, United States",
      hours: "Jumat: 11:00 - 19:00, Sabtu: 09:00 - 18:00, Minggu: 09:00 - 16:00 EDT",
      requirements: [
        "Paspor Asli / Kartu Identitas Resmi Berfoto",
        "B.A.A. Athletes' Village Number Pick-Up Pass (Barcode Digital/Cetak)",
        "Surat Konfirmasi Kualifikasi Waktu Resmi (Qualifier Proof)",
      ],
    },
    schedule: [
      { time: "06:00 EDT", activity: "Athletes' Village B.A.A. di Hopkinton High School Dibuka" },
      { time: "09:02 EDT", activity: "Start Divisi Kursi Roda Putra (Wheelchair Division)", category: "WHEELCHAIR" },
      { time: "09:05 EDT", activity: "Start Divisi Kursi Roda Putri", category: "WHEELCHAIR" },
      { time: "09:30 EDT", activity: "Start Divisi Handcycle & Atlet Disabilitas", category: "PARA" },
      { time: "09:37 EDT", activity: "Start Elite Men & Wave 1 (Hopkinton Main St)", category: "BAA 42K" },
      { time: "09:47 EDT", activity: "Start Elite Women", category: "BAA 42K" },
      { time: "10:00 EDT", activity: "Start Wave 2", category: "BAA 42K" },
      { time: "10:25 EDT", activity: "Start Wave 3", category: "BAA 42K" },
      { time: "10:50 EDT", activity: "Start Wave 4", category: "BAA 42K" },
      { time: "17:30 EDT", activity: "Penutupan Resmi Lintasan Boylston Street" },
    ],
    faqs: [
      {
        q: "Bagaimana syarat kualifikasi waktu untuk mendaftar Boston Marathon?",
        a: "Peserta wajib memiliki catatan waktu marathon resmi bersertifikasi World Athletics/AIMS/USATF sesuai batas usia (misalnya Pria 18-34: 3:00:00, Wanita 18-34: 3:30:00) yang diraih dalam periode jendela kualifikasi resmi.",
      },
      {
        q: "Apakah transportasi shuttle bus atlet ke garis start Hopkinton disediakan panitia?",
        a: "Ya, panitia B.A.A. menyediakan ratusan armada official bus gratis dari Boston Common menuju Athletes' Village di Hopkinton pada Senin pagi mulai pukul 06:00 EDT sesuai alokasi Wave pelari.",
      },
      {
        q: "Bagaimana cara penentuan alokasi nomor BIB dan Wave start?",
        a: "Nomor BIB pelari diurutkan secara ketat dan transparan berdasarkan catatan waktu kualifikasi tercepat. Pelari dengan catatan kualifikasi terbaik mendapatkan BIB angka kecil dan diberangkatkan pada Wave 1 Corral 1.",
      },
    ],
    sponsors: [
      { name: "Bank of America", role: "Presenting Partner" },
      { name: "adidas", role: "Official Footwear & Apparel" },
      { name: "Abbott World Marathon Majors", role: "Official Series" },
      { name: "Samuel Adams", role: "Official Beer" },
      { name: "Schneider Electric", role: "Sustainability Partner" },
      { name: "New England Honda", role: "Official Vehicle" },
      { name: "Maurten", role: "Official Sports Fuel" },
      { name: "Poland Spring", role: "Official Natural Spring Water" },
      { name: "Shokz", role: "Official Headphones" },
      { name: "JetBlue", role: "Official Airline" },
      { name: "Gatorade", role: "Official Sports Drink" },
      { name: "CITGO", role: "Official Landmark Partner" },
    ],
    layoutConfig: {
      templateId: "template-baa",
      heroAlignment: "center",
      heroVariant: "cinematic",
      themeStyle: "custom",
      customThemePrimary: "#002244",
      customThemeAccent: "#fdda24",
      paperTexture: false,
      showTicker: true,
      tickerText: "130TH BOSTON MARATHON PRESENTED BY BANK OF AMERICA · MONDAY, APRIL 20, 2026 · HOPKINTON TO BOYLSTON STREET · TO FINISH HERE IS EVERYTHING",
      showCountdown: true,
      showCategories: true,
      showRoute: true,
      showRpc: true,
      showSchedule: true,
      showFaq: true,
      showSponsors: true,
      showCustomBlocks: true,
      categorySectionTitle: "KATEGORI LOMBA & STANDAR KUALIFIKASI",
      categorySectionSubtitle: "Pendaftaran resmi 130th Boston Marathon dengan standar waktu kualifikasi dan nomor resmi B.A.A.",
      routeSectionTitle: "RUTE TITIK-KE-TITIK: HOPKINTON KE BOSTON",
      routeSectionSubtitle: "Profil elevasi historis 42.195 KM melintasi 8 kota New England, Heartbreak Hill, dan finish Boylston Street.",
      rpcSectionTitle: "FAN FEST & NUMBER PICK-UP (HYNES CONVENTION CENTER)",
      rpcSectionSubtitle: "Pengambilan BIB nomor dada, adidas Celebration Jacket voucher, dan kartu akses Athletes' Village.",
      scheduleSectionTitle: "PATRIOTS' DAY TIMELINE & WAVE FLAG-OFF",
      scheduleSectionSubtitle: "Jadwal pemberangkatan gelombang atlet dari Hopkinton Main Street.",
      ctaButtonText: "DAFTAR SEKARANG",
      customBlocks: [
        {
          id: "block-baa-standards",
          title: "Standar Waktu Kualifikasi Resmi B.A.A. (Qualifying Standards)",
          badge: "QUALIFIER HUB",
          style: "banner",
          content: "Semua pendaftar nomor marathon penuh wajib memenuhi standar waktu resmi sesuai kelompok usia di lintasan tersertifikasi World Athletics / AIMS / USATF. Pria 18-34: 3:00:00 | Wanita 18-34: 3:30:00 | Pria 35-39: 3:05:00 | Wanita 35-39: 3:35:00.",
        },
        {
          id: "block-baa-athletes-village",
          title: "Athletes' Village Hopkinton & Shuttle Bus Boston Common",
          badge: "LOGISTIK LOMBA",
          style: "notice",
          content: "Shuttle bus resmi B.A.A. mengantar pelari dari Boston Common ke Athletes' Village di Hopkinton High School secara gratis mulai pukul 06:00 EDT. Fasilitas mencakup tenda pemanasan, nutrisi Maurten, dan drop bag resmi.",
        },
      ],
      baaConfig: {
        presentedByText: "Presented by Bank of America",
        topAlertText: "2027 Boston Marathon Qualifier Registration is Open · Login to Athletes' Village to Apply",
        topAlertLinkText: "Login to Athletes' Village to Apply",
        topAlertLinkUrl: "https://baa.my.site.com/s/login/?language=en_US",
        unicornBadgeText: "B.A.A.",
        heroLargePhotoUrl: "/images/events/boston_marathon.jpg",
        finishLinePhotoUrl: "/images/events/boston_finish.jpg",
        editionText: "130TH BOSTON MARATHON",
        sloganText: "TO FINISH HERE IS EVERYTHING",
      },
      navbarConfig: {
        brandBadgeText: "BAA",
        brandIconUrl: "/images/events/boston_baa_logo.png",
        brandTitle: "Boston Marathon",
        brandSubtitle: "Presented by Bank of America",
      },
    },
    updatedAt: new Date().toISOString(),
  },
];

const STORAGE_KEY = "ivy_platform_marathon_events";

export function getMarathonEvents(): MarathonEvent[] {
  let list = SEED_MARATHON_EVENTS;
  if (typeof window !== "undefined") {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(SEED_MARATHON_EVENTS));
      } else {
        const parsed = JSON.parse(raw) as MarathonEvent[];
        if (Array.isArray(parsed) && parsed.length > 0) {
          // Auto-merge any missing seed events (e.g. newly added Bangkok, Ironman, & BAA templates)
          const existingIds = new Set(parsed.map((e) => e.id));
          const missingSeeds = SEED_MARATHON_EVENTS.filter((s) => !existingIds.has(s.id));
          if (missingSeeds.length > 0) {
            list = [...parsed, ...missingSeeds];
            localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
          } else {
            list = parsed;
          }
        }
      }
    } catch {
      list = SEED_MARATHON_EVENTS;
    }
  }

  // Ensure every event has complete layoutConfig with authentic templateId
  return list.map((ev) => {
    const inferredTemplateId: MarathonTemplateId =
      ev.layoutConfig?.templateId ||
      (ev.id.includes("summarecon") || ev.id.includes("bandung") ? "template-summarecon" :
       ev.id.includes("tokyo") ? "template-tokyo" :
       ev.id.includes("berlin") ? "template-berlin" :
       ev.id.includes("borobudur") ? "template-borobudur" :
       ev.id.includes("sydney") ? "template-sydney" :
       ev.id.includes("bangkok") || ev.id.includes("bkk") ? "template-bangkok" :
       ev.id.includes("ironman") || ev.id.includes("triathlon") ? "template-ironman" :
       ev.id.includes("boston") || ev.id.includes("baa") ? "template-baa" : "template-custom");

    return {
      ...ev,
      customFormConfig: { ...DEFAULT_FORM_CONFIG, ...(ev.customFormConfig || {}) },
      layoutConfig: {
        ...DEFAULT_LAYOUT_CONFIG,
        ...(ev.layoutConfig || {}),
        templateId: inferredTemplateId,
        customBlocks:
          ev.layoutConfig?.customBlocks && ev.layoutConfig.customBlocks.length > 0
            ? ev.layoutConfig.customBlocks
            : DEFAULT_LAYOUT_CONFIG.customBlocks,
      },
    };
  });
}

function mapDbEventToMarathonEvent(dbEv: any, existing?: MarathonEvent): MarathonEvent {
  const mappedCategories: MarathonCategory[] = Array.isArray(dbEv.categories) && dbEv.categories.length > 0
    ? dbEv.categories.map((c: any) => {
        const catName = c.name || "Category";
        const catLower = catName.toLowerCase();
        let dist = "10.000 KM";
        let distKm = 10.0;
        let code = c.code || "10K";

        if (catLower.includes("42") || catLower.includes("full")) {
          dist = "42.195 KM";
          distKm = 42.195;
          code = c.code || "FM";
        } else if (catLower.includes("21") || catLower.includes("half")) {
          dist = "21.0975 KM";
          distKm = 21.0975;
          code = c.code || "HM";
        } else if (catLower.includes("10")) {
          dist = "10.000 KM";
          distKm = 10.0;
          code = c.code || "10K";
        } else if (catLower.includes("5")) {
          dist = "5.000 KM";
          distKm = 5.0;
          code = c.code || "5K";
        } else if (catLower.includes("25")) {
          dist = "25.000 KM";
          distKm = 25.0;
          code = c.code || "25K";
        } else if (catLower.includes("50")) {
          dist = "50.000 KM";
          distKm = 50.0;
          code = c.code || "50K";
        }

        const mode = c.registrationMode || c.mode || "NORMAL";
        let status: MarathonCategory["status"] = "OPEN";
        if (mode === "BALLOT") status = "BALLOT_OPEN";
        else if (mode === "WAR_QUEUE") status = "WAR_SOON";

        const quota = c.capacity ?? c.quota ?? 1000;
        const price = c.price ?? 500000;

        return {
          id: c.id,
          code,
          name: catName,
          distance: dist,
          distanceKm: distKm,
          cutoffTime: distKm > 30 ? "07:00:00" : distKm > 15 ? "03:45:00" : "02:00:00",
          minAge: distKm > 30 ? 18 : 15,
          price,
          earlyBirdPrice: c.earlyBirdPrice || undefined,
          quota,
          registeredCount: c.allocatedQuota ?? 0,
          mode: mode as any,
          status,
          opensAt: c.registrationOpensAt || "2026-01-01T00:00:00Z",
          closesAt: c.registrationClosesAt || "2026-12-31T23:59:59Z",
          includes: ["Nomor Dada BIB Ber-chip", "Jersey Peserta Resmi", "Medali Finisher Eksklusif", "Asuransi Keselamatan", "Refreshment & Recovery Station"],
        };
      })
    : (existing?.categories || []);

  const cleanBanner = dbEv.bannerUrl ? dbEv.bannerUrl.replace(/^http:\/\/localhost:8081\/media\//, "") : "";
  const cleanLogo = dbEv.logoUrl ? dbEv.logoUrl.replace(/^http:\/\/localhost:8081\/media\//, "") : "";

  const bannerUrl = cleanBanner || existing?.bannerUrl || "/images/events/hero_marathon.jpg";
  const logoUrl = cleanLogo || existing?.logoUrl || "/images/ivy-icon.svg";

  const inferredTemplateId: MarathonTemplateId =
    existing?.layoutConfig?.templateId ||
    (dbEv.slug?.includes("summarecon") || dbEv.slug?.includes("bandung") ? "template-summarecon" :
     dbEv.slug?.includes("tokyo") ? "template-tokyo" :
     dbEv.slug?.includes("berlin") ? "template-berlin" :
     dbEv.slug?.includes("borobudur") ? "template-borobudur" :
     dbEv.slug?.includes("sydney") ? "template-sydney" :
     dbEv.slug?.includes("bangkok") || dbEv.slug?.includes("bkk") ? "template-bangkok" :
     dbEv.slug?.includes("ironman") || dbEv.slug?.includes("triathlon") ? "template-ironman" :
     dbEv.slug?.includes("boston") || dbEv.slug?.includes("baa") ? "template-baa" : "template-custom");

  return {
    id: dbEv.id,
    slug: dbEv.slug || existing?.slug || dbEv.id,
    name: dbEv.name,
    tagline: existing?.tagline || dbEv.description || "",
    description: dbEv.description || existing?.description || "",
    eventType: dbEv.eventType || existing?.eventType || "MARATHON",
    status: (dbEv.status as any) || existing?.status || "published",
    organizerName: dbEv.organizerName || existing?.organizerName || "Ivy Sports Management",
    organizerSlug: dbEv.organizerSlug || existing?.organizerSlug || "ivy-sports",
    city: existing?.city || (dbEv.venueAddress?.split(",").slice(-2, -1)[0]?.trim()) || "Indonesia",
    province: existing?.province || (dbEv.venueAddress?.split(",").slice(-1)[0]?.trim()) || "Indonesia",
    country: existing?.country || "Indonesia",
    venueName: dbEv.venueName || existing?.venueName || "",
    venueAddress: dbEv.venueAddress || existing?.venueAddress || "",
    startsAt: dbEv.startsAt || existing?.startsAt || new Date().toISOString(),
    flagOffTime: existing?.flagOffTime || "05:00 WIB",
    bannerUrl,
    logoUrl,
    themeColor: existing?.themeColor || "#ea580c",
    accentColor: existing?.accentColor || "#ea580c",
    certifiedBy: existing?.certifiedBy || "World Athletics & PASI",
    elevationGain: existing?.elevationGain || "+120m",
    waterStationsCount: existing?.waterStationsCount || 18,
    medicalStationsCount: existing?.medicalStationsCount || 8,
    categories: mappedCategories,
    routeHighlights: existing?.routeHighlights || ["Start / Finish Gate Resmi", "Water Station Setiap 2.5 KM", "Zona Medis & Ambulans Standar AIMS", "Cheer Zone Budaya & Musik"],
    racepackInfo: existing?.racepackInfo || {
      dates: "H-2 sampai H-1 Lomba",
      venue: dbEv.venueName || "Race Village",
      address: dbEv.venueAddress || "Race Village Expo",
      hours: "10:00 - 20:00 WIB",
      requirements: ["Email Konfirmasi & QR Code", "Kartu Identitas KTP/Paspor Asli", "Surat Keterangan Sehat Fisik"],
    },
    schedule: existing?.schedule || [
      { time: "04:00 WIB", activity: "Race Village & Bag Drop Open" },
      { time: "04:30 WIB", activity: "Line Up & National Anthem" },
      { time: "05:00 WIB", activity: "Flag-off Resmi" },
      { time: "09:30 WIB", activity: "Podium Ceremony & Awarding" },
    ],
    faqs: existing?.faqs || [
      { q: "Apakah tiket dapat dipindahtangankan?", a: "Sesuai regulasi PASI dan World Athletics, tiket bersifat personal dan tidak dapat dipindahtangankan tanpa persetujuan panitia." },
      { q: "Bagaimana sistem pengambilan Race Pack?", a: "Pengambilan Race Pack wajib membawa bukti konfirmasi pendaftaran dan kartu identitas resmi." },
    ],
    sponsors: existing?.sponsors || [
      { name: "Bank of America", role: "Presenting Partner" },
      { name: "adidas", role: "Official Footwear & Apparel" },
      { name: "Abbott World Marathon Majors", role: "Official Series" },
    ],
    customFormConfig: { ...DEFAULT_FORM_CONFIG, ...(existing?.customFormConfig || {}) },
    layoutConfig: {
      ...DEFAULT_LAYOUT_CONFIG,
      ...(existing?.layoutConfig || {}),
      templateId: inferredTemplateId,
    },
    updatedAt: dbEv.updatedAt || new Date().toISOString(),
  };
}

export async function syncMarathonEventsFromServer(): Promise<MarathonEvent[]> {
  try {
    const dbEvents = await fetchPublicEvents();
    if (!Array.isArray(dbEvents) || dbEvents.length === 0) {
      return getMarathonEvents();
    }

    const currentLocal = getMarathonEvents();
    const mergedList: MarathonEvent[] = [];
    const matchedLocalIds = new Set<string>();

    for (const dbEv of dbEvents) {
      const existing = currentLocal.find(
        (e) => e.id === dbEv.id || e.slug === dbEv.slug || (dbEv.slug && e.id.includes(dbEv.slug)) || (e.slug && dbEv.id.includes(e.slug))
      );
      if (existing) {
        matchedLocalIds.add(existing.id);
      }
      mergedList.push(mapDbEventToMarathonEvent(dbEv, existing));
    }

    for (const loc of currentLocal) {
      if (!matchedLocalIds.has(loc.id) && !mergedList.some((m) => m.id === loc.id || m.slug === loc.slug)) {
        mergedList.push(loc);
      }
    }

    if (typeof window !== "undefined") {
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(mergedList));
        window.dispatchEvent(new CustomEvent("ivy:events_changed", { detail: mergedList }));
      } catch {}
    }

    return mergedList;
  } catch (err) {
    console.error("Failed to sync marathon events from server:", err);
    return getMarathonEvents();
  }
}

export async function fetchMarathonEventByIdFromServer(idOrSlug: string): Promise<MarathonEvent | null> {
  try {
    const dbEv = await fetchPublicEvent(idOrSlug);
    if (!dbEv || !dbEv.id) {
      return getMarathonEventById(idOrSlug) || null;
    }
    const currentLocal = getMarathonEvents();
    const existing = currentLocal.find(
      (e) => e.id === dbEv.id || e.slug === dbEv.slug || (dbEv.slug && e.id.includes(dbEv.slug))
    );
    const mapped = mapDbEventToMarathonEvent(dbEv, existing);

    saveMarathonEvent(mapped);
    return mapped;
  } catch {
    return getMarathonEventById(idOrSlug) || null;
  }
}

export function getMarathonEventById(idOrSlug: string): MarathonEvent | undefined {
  const events = getMarathonEvents();
  return events.find(
    (e) => e.id === idOrSlug || e.slug === idOrSlug || e.id.includes(idOrSlug) || idOrSlug.includes(e.id)
  );
}

export function saveMarathonEvent(updated: MarathonEvent): void {
  if (typeof window === "undefined") return;
  try {
    const events = getMarathonEvents();
    const idx = events.findIndex((e) => e.id === updated.id);
    updated.updatedAt = new Date().toISOString();
    if (idx >= 0) {
      events[idx] = updated;
    } else {
      events.push(updated);
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(events));
    window.dispatchEvent(
      new CustomEvent("ivy:events_changed", { detail: events })
    );
  } catch {
    // quota
  }
}

export function deleteMarathonEvent(id: string): void {
  if (typeof window === "undefined") return;
  try {
    const events = getMarathonEvents().filter(e => e.id !== id);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(events));
    window.dispatchEvent(
      new CustomEvent("ivy:events_changed", { detail: events })
    );
  } catch {
    // quota or error
  }
}

export function resetMarathonEvents(): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(SEED_MARATHON_EVENTS));
  window.dispatchEvent(
    new CustomEvent("ivy:events_changed", { detail: SEED_MARATHON_EVENTS })
  );
}

export interface MarathonTemplatePreset {
  id: MarathonTemplateId;
  name: string;
  source: string;
  badge: string;
  badgeClass: string;
  description: string;
  defaultBanner: string;
  themeColor: string;
  accentColor: string;
  certifiedBy: string;
  elevationGain: string;
  sampleCategories: MarathonCategory[];
  sampleSchedule: Array<{ time: string; activity: string; category?: string }>;
  sampleHighlights: string[];
  defaultLayout: MarathonLayoutConfig;
}

export const MARATHON_TEMPLATES: MarathonTemplatePreset[] = [
  {
    id: "template-summarecon",
    name: "Summarecon Bandung Run Fest",
    source: "summareconbandungrunfest.com",
    badge: "Urban Fest & Neon",
    badgeClass: "bg-emerald-500/20 text-emerald-300 border-emerald-500/30",
    description: "Template festival lari kota dengan tipografi athletic bold, seksi kuning 'New Roads New Possibilities', kartu kategori hitam bernomor 01-04 dengan tombol 'MORE INFO' modal, banner penutup 'The Road Is Finally Yours', dan tekstur kertas warm paper.",
    defaultBanner: "/images/events/bandung_runfest.jpg",
    themeColor: "#2EAF4A",
    accentColor: "#F5EB15",
    certifiedBy: "PASI & World Athletics Measured",
    elevationGain: "15m (Fast & Flat Course)",
    sampleCategories: [
      {
        id: "cat-smr-10k",
        code: "10K",
        name: "10K Open & Master",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:15:00",
        minAge: 16,
        price: 375000,
        earlyBirdPrice: 300000,
        quota: 3000,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official Running Tee", "Medali Finisher 10K", "BIB Timing Chip RFID", "Refreshment"],
      },
      {
        id: "cat-smr-5k",
        code: "5K",
        name: "5K Fun Run",
        distance: "5.000 KM",
        distanceKm: 5.0,
        cutoffTime: "01:15:00",
        minAge: 12,
        price: 275000,
        quota: 4000,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official Running Tee", "Medali Finisher 5K", "BIB Number", "Refreshment"],
      },
      {
        id: "cat-smr-kids",
        code: "KIDS",
        name: "Kids Dash 1.5K",
        distance: "1.500 KM",
        distanceKm: 1.5,
        cutoffTime: "00:45:00",
        minAge: 5,
        price: 180000,
        quota: 1000,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Jersey Anak Spesial", "Medali Emas Anak", "Paket Snack"],
      },
    ],
    sampleSchedule: [
      { time: "05:00 WIB", activity: "Senam Pemanasan & Zumba Bersama" },
      { time: "05:30 WIB", activity: "Flag-Off 10K Open & Master", category: "10K" },
      { time: "06:00 WIB", activity: "Flag-Off 5K Fun Run", category: "5K" },
      { time: "06:45 WIB", activity: "Flag-Off Kids Dash 1.5K", category: "KIDS" },
      { time: "08:15 WIB", activity: "Live Music & Doorprize Utama" },
    ],
    sampleHighlights: [
      "Boulevard Utama Summarecon Bandung Steril dan Mulus",
      "Zona Hidrasi Dingin & Water Mist Shower",
      "Race Village Interaktif dengan Bazar Kuliner & Panggung Musik",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-summarecon",
      heroAlignment: "left",
      heroVariant: "poster",
      themeStyle: "summarecon_neon",
      paperTexture: true,
      showTicker: true,
      tickerText: "NEW ROADS · NEW POSSIBILITIES · FAST COURSE · SUMMARECON BANDUNG",
    },
  },
  {
    id: "template-tokyo",
    name: "Tokyo International Marathon",
    source: "marathon.tokyo",
    badge: "Abbott World Major",
    badgeClass: "bg-red-500/20 text-red-300 border-red-500/30",
    description: "Template presisi ala Abbott World Marathon Majors Jepang. Desain kontras tinggi navy & crimson red, tipografi bilingual Tokyo, tabel breakdown kuota ballot 38.000 pelari, timer countdown digital bergaris, dan expo Tokyo Big Sight.",
    defaultBanner: "/images/events/tokyo_marathon.jpg",
    themeColor: "#dc2626",
    accentColor: "#1e3a8a",
    certifiedBy: "Abbott World Marathon Majors Platinum",
    elevationGain: "35m (Flat World Record Course)",
    sampleCategories: [
      {
        id: "cat-tky-fm",
        code: "FM42K",
        name: "Tokyo Full Marathon",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "07:00:00",
        minAge: 19,
        price: 1250000,
        quota: 37500,
        registeredCount: 0,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official Asics Singlet", "Finisher Robe Handuk", "Medali Cor Tokyo Major", "BIB RFID Chip", "Asuransi"],
      },
      {
        id: "cat-tky-10k",
        code: "10.7K",
        name: "10.7K Junior & Wheelchair",
        distance: "10.700 KM",
        distanceKm: 10.7,
        cutoffTime: "01:45:00",
        minAge: 16,
        price: 550000,
        quota: 500,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official Asics Singlet", "Medali Finisher 10.7K", "BIB RFID Chip"],
      },
    ],
    sampleSchedule: [
      { time: "07:00 JST", activity: "Assembly Gate Closes & Security Check" },
      { time: "09:05 JST", activity: "Wheelchair Marathon Start", category: "WHEELCHAIR" },
      { time: "09:10 JST", activity: "Marathon Wave 1 Start (Elite & Corrals A-D)", category: "FM42K" },
      { time: "09:30 JST", activity: "Marathon Wave 2 Start (Corrals E-K)", category: "FM42K" },
      { time: "16:10 JST", activity: "Course Finish Cut-Off & Closing" },
    ],
    sampleHighlights: [
      "Tokyo Metropolitan Government Building Shinjuku Start",
      "Asakusa Kaminarimon Gate & Ginza District Historic Course",
      "Finish Line di Depan Stasiun Tokyo (Gyoko-dori Avenue)",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-tokyo",
      heroAlignment: "center",
      heroVariant: "centered",
      themeStyle: "tokyo_platinum",
      paperTexture: false,
      showTicker: true,
      tickerText: "THE DAY WE UNITE · TOKYO MARATHON · ABBOTT WORLD MARATHON MAJORS",
    },
  },
  {
    id: "template-berlin",
    name: "BMW Berlin Speed Marathon",
    source: "bmw-berlin-marathon.com",
    badge: "World Record Fastest",
    badgeClass: "bg-sky-500/20 text-sky-300 border-sky-500/30",
    description: "Template lintasan rekor dunia tercepat. Layout split-screen berdampingan, garis kecepatan BMW Blue, grafik elevasi flat 20m, kategori inline skate/handbike/running, dan klimaks Gerbang Brandenburg KM 41.8.",
    defaultBanner: "/images/events/hero_marathon.jpg",
    themeColor: "#0066b2",
    accentColor: "#00b2e3",
    certifiedBy: "Abbott World Marathon Majors Platinum",
    elevationGain: "20m (Flat Course Record)",
    sampleCategories: [
      {
        id: "cat-ber-fm",
        code: "FM42K",
        name: "BMW Berlin Marathon Running",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "06:15:00",
        minAge: 18,
        price: 1350000,
        quota: 45000,
        registeredCount: 0,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Adidas Running Singlet", "Finisher Tee Khusus", "Medali Edisi Rekor Dunia", "ChampionChip Timing", "Poncho Hangat"],
      },
      {
        id: "cat-ber-inline",
        code: "INLINE",
        name: "Berlin Inline Skating Marathon",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "02:30:00",
        minAge: 17,
        price: 850000,
        quota: 5000,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "WAR_SOON",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Skater Race Tee", "Medali Finisher Logam", "BIB RFID Chip"],
      },
      {
        id: "cat-ber-handbike",
        code: "HANDBIKE",
        name: "Handbike & Wheelchair 42K",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "03:00:00",
        minAge: 16,
        price: 650000,
        quota: 1000,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official Jersey", "Medali Finisher", "BIB RFID", "Technical Support"],
      },
    ],
    sampleSchedule: [
      { time: "08:50 CET", activity: "Handbike & Wheelchair Start", category: "HANDBIKE" },
      { time: "09:15 CET", activity: "Wave 1 Running Start (Elite & Corrals A-D)", category: "FM42K" },
      { time: "09:45 CET", activity: "Wave 2 Running Start (Corrals E-F)", category: "FM42K" },
      { time: "10:10 CET", activity: "Wave 3 Running Start (Corrals G-H)", category: "FM42K" },
      { time: "15:30 CET", activity: "Finish Line Closes di Strasse des 17. Juni" },
    ],
    sampleHighlights: [
      "Lintasan Rekor Dunia Tercepat Tanpa Tanjakan Terjal",
      "Melintasi Sudut Bersejarah Berlin Timur & Barat",
      "Puncak Emosional Melewati Gerbang Brandenburg 200m Menjelang Finish",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-berlin",
      heroAlignment: "left",
      heroVariant: "split",
      themeStyle: "berlin_speed",
      paperTexture: false,
      showTicker: true,
      tickerText: "BERLIN LEGENDS · THE FASTEST MARATHON ON PLANET EARTH · 42.195 KM",
    },
  },
  {
    id: "template-borobudur",
    name: "Borobudur Heritage Marathon",
    source: "borobudurmarathon.com",
    badge: "Cultural Heritage",
    badgeClass: "bg-orange-500/20 text-orange-300 border-orange-500/30",
    description: "Template kultural warisan budaya dunia. Latar fajar Candi Borobudur dengan warna terakota & emas antik, tajuk 'Decade of Legacy / Dasa Warsa Warisan', rute rolling hills Menoreh (285m), 15 cheer zone kesenian warga 12 desa, dan Pasar Medang.",
    defaultBanner: "/images/events/borobudur_marathon.jpg",
    themeColor: "#ea580c",
    accentColor: "#ffa800",
    certifiedBy: "World Athletics & PASI",
    elevationGain: "285m (Scenic Rolling Hills Menoreh)",
    sampleCategories: [
      {
        id: "cat-brb-fm",
        code: "FM42K",
        name: "Full Marathon Heritage",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "07:00:00",
        minAge: 18,
        price: 850000,
        quota: 3000,
        registeredCount: 0,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Jersey Motif Batik Borobudur", "Finisher Tee Khusus FM", "Medali Cor Kultural", "BIB RFID Chip", "Asuransi"],
      },
      {
        id: "cat-brb-hm",
        code: "HM21K",
        name: "Half Marathon Heritage",
        distance: "21.097 KM",
        distanceKm: 21.097,
        cutoffTime: "03:45:00",
        minAge: 17,
        price: 650000,
        quota: 4500,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "WAR_SOON",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Jersey Motif Batik Borobudur", "Medali Cor Kultural", "BIB RFID Chip", "Refreshment"],
      },
      {
        id: "cat-brb-10k",
        code: "10K",
        name: "10K Heritage Run",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:00:00",
        minAge: 15,
        price: 450000,
        quota: 2500,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Jersey Motif Batik Borobudur", "Medali Cor Kultural", "BIB RFID Chip"],
      },
    ],
    sampleSchedule: [
      { time: "04:00 WIB", activity: "Gate Opening & Doa Bersama di Kompleks Candi" },
      { time: "04:30 WIB", activity: "Flag-Off Full Marathon (COT 7 Jam)", category: "FM42K" },
      { time: "05:15 WIB", activity: "Flag-Off Half Marathon (COT 3.5 Jam)", category: "HM21K" },
      { time: "06:00 WIB", activity: "Flag-Off 10K Run (COT 2 Jam)", category: "10K" },
      { time: "11:30 WIB", activity: "Penutupan Rute & Pesta Budaya Magelang di Pasar Medang" },
    ],
    sampleHighlights: [
      "Latar Fajar Keemasan Menoreh & Candi Borobudur Abad ke-8",
      "Sambutan Semarak 15 Cheer Zone Kesenian Warga dari 12 Desa Tradisional",
      "Pasar Medang: Kuliner UMKM Otentik & Cendera Mata Finisher",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-borobudur",
      heroAlignment: "center",
      heroVariant: "cinematic",
      themeStyle: "borobudur_heritage",
      paperTexture: true,
      showTicker: true,
      tickerText: "DECADE OF LEGACY · DASA WARSA WARISAN · RUN WITH HEART IN BOROBUDUR",
    },
  },
  {
    id: "template-sydney",
    name: "TCS Sydney Harbour Marathon",
    source: "sydneymarathon.com",
    badge: "Harbour Major",
    badgeClass: "bg-teal-500/20 text-teal-300 border-teal-500/30",
    description: "Template maritim spektakuler Harbour Bridge hingga Sydney Opera House. Desain biru laut pesisir pasifik, pembagian assembly area wave warna (Purple, Green, Orange, High Performance), dan angin sepoi pesisir.",
    defaultBanner: "/images/events/hero_marathon.jpg",
    themeColor: "#0284c7",
    accentColor: "#0f766e",
    certifiedBy: "World Athletics Platinum Candidate",
    elevationGain: "115m (Scenic Harbour Course)",
    sampleCategories: [
      {
        id: "cat-syd-fm",
        code: "FM42K",
        name: "TCS Sydney Marathon",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "07:00:00",
        minAge: 18,
        price: 1200000,
        quota: 25000,
        registeredCount: 0,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["ASICS Official Singlet", "Finisher Tee", "Medali Cor Sydney Opera House", "BIB RFID Chip", "Tiket Transportasi NSW Gratis"],
      },
      {
        id: "cat-syd-hm",
        code: "HM21K",
        name: "Sydney Half Marathon",
        distance: "21.097 KM",
        distanceKm: 21.097,
        cutoffTime: "03:30:00",
        minAge: 16,
        price: 750000,
        quota: 10000,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "WAR_SOON",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official Running Singlet", "Medali Finisher 21K", "BIB RFID Chip", "NSW Transport Pass"],
      },
      {
        id: "cat-syd-10k",
        code: "10K",
        name: "Sydney 10K Bridge Run",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:00:00",
        minAge: 12,
        price: 500000,
        quota: 5000,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official Running Tee", "Medali Finisher 10K", "BIB RFID Number"],
      },
    ],
    sampleSchedule: [
      { time: "05:30 AEST", activity: "Bradfield Park Milsons Point Assembly Open" },
      { time: "06:00 AEST", activity: "Wheelchair Elite Flag-off", category: "WHEELCHAIR" },
      { time: "06:05 AEST", activity: "Wave 1 Start: High Performance & Purple", category: "FM42K" },
      { time: "06:25 AEST", activity: "Wave 2 Start: Green Corrals", category: "FM42K" },
      { time: "06:50 AEST", activity: "Wave 3 Start: Orange Corrals", category: "FM42K" },
      { time: "13:00 AEST", activity: "Finish Line Celebration di Sydney Opera House Forecourt" },
    ],
    sampleHighlights: [
      "Lari Menyeberangi Ikon Sydney Harbour Bridge yang Dikosongkan Khusus",
      "Udara Segar Samudra Pasifik Melintasi Royal Botanic Garden",
      "Finish Line Ikonik di Opera House Forecourt dengan Pemandangan Laut",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-sydney",
      heroAlignment: "left",
      heroVariant: "poster",
      themeStyle: "berlin_speed",
      paperTexture: false,
      showTicker: true,
      tickerText: "RUN ACROSS THE HARBOUR BRIDGE · FINISH AT SYDNEY OPERA HOUSE",
    },
  },
  {
    id: "template-bangkok",
    name: "Bangkok Midnight Marathon (BDMS)",
    source: "bkkmarathon.com",
    badge: "Midnight Grand Palace",
    badgeClass: "bg-amber-500/20 text-amber-300 border-amber-500/30",
    description: "Template lari malam tropis Sanam Chai Grand Palace & Jembatan Kabel Rama VIII. Kartu tiket bergaya sobekan sirkular (card-ticket side cutouts), badge miring skew-box (-15 deg), tombol 3D border-bottom, dan tabel jadwal bersayap melengkung dengan aksen Fire Orange & Thai Amber.",
    defaultBanner: "/images/events/bangkok_marathon.jpg",
    themeColor: "#f45227",
    accentColor: "#ed9227",
    certifiedBy: "World Athletics & AIMS Label Road Race",
    elevationGain: "45m (Rama VIII Bridge & Elevated Skyway)",
    sampleCategories: [
      {
        id: "cat-bkk-fm",
        code: "FM42K",
        name: "Full Marathon Midnight",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "06:00:00",
        minAge: 18,
        price: 1450000,
        quota: 5000,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official BDMS Singlet", "Finisher Long Sleeve Tee", "Medali Cor Emas", "BIB RFID Chip", "Tropical Night Refreshment"],
      },
      {
        id: "cat-bkk-hm",
        code: "HM21K",
        name: "Half Marathon Midnight",
        distance: "21.097 KM",
        distanceKm: 21.097,
        cutoffTime: "03:30:00",
        minAge: 16,
        price: 1150000,
        quota: 8000,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official BDMS Singlet", "Medali Finisher 21K", "BIB RFID Chip"],
      },
      {
        id: "cat-bkk-10k",
        code: "10K",
        name: "Mini Marathon 10K",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:00:00",
        minAge: 14,
        price: 850000,
        quota: 10000,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official Event Tee", "Medali Finisher 10K", "BIB RFID"],
      },
    ],
    sampleSchedule: [
      { time: "23:00 ICT", activity: "Race Village Sanam Chai Assembly & Drop Bag Open" },
      { time: "00:30 ICT", activity: "Flag-Off Full Marathon 42.195 KM (COT 6 Jam)", category: "FM42K" },
      { time: "03:00 ICT", activity: "Flag-Off Half Marathon 21.097 KM (COT 3.5 Jam)", category: "HM21K" },
      { time: "04:30 ICT", activity: "Flag-Off Mini Marathon 10 KM (COT 2 Jam)", category: "10K" },
      { time: "06:30 ICT", activity: "Grand Palace Sunrise Finisher Ceremony & Closing" },
    ],
    sampleHighlights: [
      "Start Tengah Malam 00:30 ICT di Depan Grand Palace Sanam Chai Road",
      "Melintasi Kemegahan Jembatan Kabel Rama VIII di Atas Sungai Chao Phraya",
      "Jalan Layang Elevated Highway Bebas Hambatan dan Sejuk",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-bangkok",
      heroAlignment: "center",
      heroVariant: "centered",
      themeStyle: "custom",
      customThemePrimary: "#f45227",
      customThemeAccent: "#ed9227",
      paperTexture: false,
      showTicker: true,
      tickerText: "THE 37TH BANGKOK MARATHON · SANAM CHAI GRAND PALACE · RAMA VIII BRIDGE · MIDNIGHT START",
      bangkokConfig: {
        runnerPhotoUrl: "/images/events/bangkok_runner_action.jpg",
        midnightCallout: "00:30 ICT · DEPAN GRAND PALACE SANAM CHAI",
        topRibbonText: "THE 37TH BANGKOK MARATHON · SANAM CHAI GRAND PALACE · RAMA VIII BRIDGE · MIDNIGHT START 00:30 ICT",
        skewBadgeText: "THE 37TH BANGKOK MARATHON (BDMS)",
      },
    },
  },
  {
    id: "template-ironman",
    name: "IRONMAN 70.3 & Full Triathlon",
    source: "ironman.com",
    badge: "Triathlon World Series",
    badgeClass: "bg-red-600/20 text-red-300 border-red-500/40",
    description: "Template multi-disiplin triathlon resmi standar IRONMAN. Menampilkan split 3 disiplin (Swim 1.9K/3.8K, Bike 90K/180K, Run 21.1K/42.2K) plus transisi T1 & T2, dasbor metrik cuaca dan suhu air, cut-off time bertingkat tiap disiplin, kuota slot World Championship Kona, dan panggung finish karpet merah.",
    defaultBanner: "/images/events/ironman_triathlon.jpg",
    themeColor: "#e10600",
    accentColor: "#171717",
    certifiedBy: "World Triathlon & IRONMAN Pro Series Official",
    elevationGain: "480m (Rolling Coastal Highway & Jungle Foothills)",
    sampleCategories: [
      {
        id: "cat-im-703-indiv",
        code: "IM 70.3",
        name: "IRONMAN 70.3 Individual",
        distance: "113.0 KM (1.9K Swim / 90K Bike / 21.1K Run)",
        distanceKm: 113.0,
        cutoffTime: "08:30:00",
        minAge: 18,
        price: 6250000,
        quota: 2500,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Finisher Polo & Backpack", "Medali Cor Berat M-Dot", "Silicone Swim Cap", "Transponder Chip T1/T2", "Banquet Access"],
      },
      {
        id: "cat-im-703-relay",
        code: "RELAY 70.3",
        name: "IRONMAN 70.3 Team Relay",
        distance: "113.0 KM (Tim Perenang, Pesepeda, Pelari)",
        distanceKm: 113.0,
        cutoffTime: "08:30:00",
        minAge: 18,
        price: 8500000,
        quota: 300,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["3 Paket Finisher Gear", "Relay Timing Transponder", "Athlete Village Access"],
      },
      {
        id: "cat-im-1406-full",
        code: "FULL 140.6",
        name: "IRONMAN Full Distance 140.6",
        distance: "226.0 KM (3.8K Swim / 180.2K Bike / 42.2K Run)",
        distanceKm: 226.0,
        cutoffTime: "17:00:00",
        minAge: 18,
        price: 11500000,
        quota: 1200,
        registeredCount: 0,
        mode: "BALLOT",
        status: "BALLOT_OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Finisher Jacket Eksklusif 140.6", "Medali Emas M-Dot Finisher", "Finisher Towel & Hat", "Slot Kona Rolldown Access"],
      },
    ],
    sampleSchedule: [
      { time: "05:00 MYT", activity: "Transition Zone (T1/T2) Opens & Tire Pumping" },
      { time: "06:30 MYT", activity: "Pro Rolling Start (Swim 1.9KM)", category: "IM 70.3" },
      { time: "08:15 MYT", activity: "Swim Cut-Off (1h 10m elapsed)" },
      { time: "13:30 MYT", activity: "Bike Intermediate Cut-Off (5h 30m elapsed)" },
      { time: "16:45 MYT", activity: "Official Finish Line Cut-Off (8h 30m total)", category: "IM 70.3" },
      { time: "18:00 MYT", activity: "VinFast IRONMAN World Championship Kona Slot Allocation" },
    ],
    sampleHighlights: [
      "Renang 1.9KM di Perairan Tenang Teluk Pesisir Samudra Tropis",
      "Sepeda 90KM Non-Drafting di Pesisir Pantai Pasir Tengkorak & Kaki Gunung",
      "Lari 21.1KM Menuju Red Carpet Finish Chute 'YOU ARE AN IRONMAN!'",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-ironman",
      heroAlignment: "left",
      heroVariant: "poster",
      themeStyle: "custom",
      customThemePrimary: "#e10600",
      customThemeAccent: "#171717",
      paperTexture: false,
      showTicker: true,
      tickerText: "VINFAST IRONMAN WORLD CHAMPIONSHIP QUALIFIER · 45 SLOTS TO KONA · YOU ARE AN IRONMAN",
      ironmanConfig: {
        swimMetric: "Ocean (1.9K)",
        bikeMetric: "Hilly (90K)",
        runMetric: "Flat (21.1K)",
        airTempMetric: "86°F / 30°C",
        waterTempMetric: "84°F / 29°C",
        konaSlots: "45 SLOTS",
      },
    },
  },
  {
    id: "template-baa",
    name: "Boston Athletic Association (B.A.A.) Marathon",
    source: "baa.org",
    badge: "B.A.A. Abbott Major",
    badgeClass: "bg-blue-900/40 text-yellow-300 border-yellow-400/40",
    description: "Template otentik B.A.A. Boston Marathon dari https://www.baa.org/. Foto besar hero Boylston Street, pita alert registrasi kualifikasi kuning, animasi pita sponsor bergerak tanpa putus (Bank of America, adidas, Abbott, Sam Adams, Honda, Maurten, Shokz, JetBlue, Gatorade), badge unicorn B.A.A., dan tabel standar waktu kualifikasi resmi.",
    defaultBanner: "/images/events/boston_marathon.jpg",
    themeColor: "#002244",
    accentColor: "#fdda24",
    certifiedBy: "World Athletics Platinum Label & Abbott World Marathon Majors",
    elevationGain: "Rolling Downhill (-135m Net Elevation Drop, Heartbreak Hill at Mile 20.5)",
    sampleCategories: [
      {
        id: "cat-baa-fm-qualifier",
        code: "BAA 42K",
        name: "Boston Marathon Qualifier (Time Standard)",
        distance: "42.195 KM (Hopkinton to Boston)",
        distanceKm: 42.195,
        cutoffTime: "06:00:00",
        minAge: 18,
        price: 3850000,
        quota: 30000,
        registeredCount: 0,
        mode: "PRIORITY_ACCESS",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official adidas Boston Marathon Celebration Jacket Voucher", "Iconic Unicorn Finisher Medal", "Athletes Village Hopkinton Access", "Boylston St Recovery Zone"],
      },
      {
        id: "cat-baa-para",
        code: "PARA",
        name: "Para Athletics Division & Wheelchair",
        distance: "42.195 KM (Handcycle & Wheelchair Division)",
        distanceKm: 42.195,
        cutoffTime: "05:00:00",
        minAge: 18,
        price: 1950000,
        quota: 500,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official adidas Gear Pack", "Unicorn Finisher Medal", "Dedicated Wave Start"],
      },
      {
        id: "cat-baa-5k",
        code: "BAA 5K",
        name: "B.A.A. 5K presented by Point32Health",
        distance: "5.000 KM (Boston Common & Back Bay)",
        distanceKm: 5.0,
        cutoffTime: "01:15:00",
        minAge: 12,
        price: 950000,
        quota: 10000,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Official B.A.A. 5K Tech Shirt", "Medali Finisher 5K", "Timing Tag"],
      },
    ],
    sampleSchedule: [
      { time: "06:00 EDT", activity: "Athletes' Village Opens at Hopkinton High School" },
      { time: "09:02 EDT", activity: "Men's Wheelchair Division Start", category: "WHEELCHAIR" },
      { time: "09:05 EDT", activity: "Women's Wheelchair Division Start", category: "WHEELCHAIR" },
      { time: "09:37 EDT", activity: "Elite Men & Wave 1 Start (Hopkinton Main St)", category: "BAA 42K" },
      { time: "09:47 EDT", activity: "Elite Women Start", category: "BAA 42K" },
      { time: "10:00 EDT", activity: "Wave 2 Start", category: "BAA 42K" },
      { time: "10:25 EDT", activity: "Wave 3 Start", category: "BAA 42K" },
      { time: "10:50 EDT", activity: "Wave 4 Start", category: "BAA 42K" },
      { time: "17:30 EDT", activity: "Official Course Closure at Boylston Street" },
    ],
    sampleHighlights: [
      "Hopkinton Town Common: Garis start legendaris lomba marathon tertua di dunia",
      "Wellesley College Scream Tunnel (Mile 13.1): Sorak sorai ribuan mahasiswi yang terdengar dari 1 mil",
      "Heartbreak Hill (Mile 20.5): Tanjakan paling ikonik setelah Newton Fire Station",
      "Right on Hereford, Left on Boylston (Mile 25.8): Tikungan keramat menuju garis finish Copley Square",
      "Garis Finish Boylston Street (Mile 26.2): Puncak pencapaian setiap pelari marathon dunia",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-baa",
      heroAlignment: "center",
      heroVariant: "cinematic",
      themeStyle: "custom",
      customThemePrimary: "#002244",
      customThemeAccent: "#fdda24",
      paperTexture: false,
      showTicker: true,
      tickerText: "130TH BOSTON MARATHON PRESENTED BY BANK OF AMERICA · MONDAY, APRIL 20, 2026 · TO FINISH HERE IS EVERYTHING",
      baaConfig: {
        presentedByText: "Presented by Bank of America",
        topAlertText: "2027 Boston Marathon Qualifier Registration is Open · Login to Athletes' Village to Apply",
        topAlertLinkText: "Login to Athletes' Village",
        topAlertLinkUrl: "https://baa.my.site.com/s/login/?language=en_US",
        unicornBadgeText: "B.A.A.",
        heroLargePhotoUrl: "/images/events/boston_marathon.jpg",
        finishLinePhotoUrl: "/images/events/boston_finish.jpg",
        editionText: "130TH BOSTON MARATHON",
        sloganText: "TO FINISH HERE IS EVERYTHING",
      },
    },
  },
  {
    id: "template-custom",
    name: "Super Custom Studio Template",
    source: "IvyTicketing Custom Engine",
    badge: "Kustom Bebas 100%",
    badgeClass: "bg-purple-500/20 text-purple-300 border-purple-500/30",
    description: "Template fleksibel penuh tanpa batas. Anda bebas menentukan posisi judul (kiri/tengah/kanan), varian hero, skema warna hex mandiri, tombol CTA, visibilitas seksi, dan menambahkan blok konten apa pun.",
    defaultBanner: "/images/events/hero_marathon.jpg",
    themeColor: "#ea580c",
    accentColor: "#f59e0b",
    certifiedBy: "PASI & World Athletics",
    elevationGain: "50m (Custom Route)",
    sampleCategories: [
      {
        id: "cat-cst-42k",
        code: "FM42K",
        name: "42K Marathon Open",
        distance: "42.195 KM",
        distanceKm: 42.195,
        cutoffTime: "06:30:00",
        minAge: 18,
        price: 850000,
        quota: 2000,
        registeredCount: 0,
        mode: "WAR_QUEUE",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Running Singlet", "Finisher Tee", "Medali Cor Kustom", "BIB RFID Chip"],
      },
      {
        id: "cat-cst-21k",
        code: "HM21K",
        name: "21K Half Marathon",
        distance: "21.097 KM",
        distanceKm: 21.097,
        cutoffTime: "03:30:00",
        minAge: 17,
        price: 600000,
        quota: 3000,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Running Singlet", "Medali Finisher 21K", "BIB RFID Chip"],
      },
      {
        id: "cat-cst-10k",
        code: "10K",
        name: "10K Road Challenge",
        distance: "10.000 KM",
        distanceKm: 10.0,
        cutoffTime: "02:00:00",
        minAge: 15,
        price: 400000,
        quota: 2000,
        registeredCount: 0,
        mode: "NORMAL",
        status: "OPEN",
        opensAt: new Date().toISOString(),
        closesAt: new Date(Date.now() + 30 * 86400000).toISOString(),
        includes: ["Running Tee", "Medali Finisher", "BIB Number"],
      },
    ],
    sampleSchedule: [
      { time: "04:30 WIB", activity: "Flag-Off Marathon 42K", category: "FM42K" },
      { time: "05:15 WIB", activity: "Flag-Off 21K Half Marathon", category: "HM21K" },
      { time: "06:00 WIB", activity: "Flag-Off 10K Challenge", category: "10K" },
      { time: "11:00 WIB", activity: "Cut-off & Prize Presentation" },
    ],
    sampleHighlights: [
      "Desain Layout Super Fleksibel",
      "Dukungan Blok Konten Tambahan Bebas",
      "Kustomisasi Formulir Data Pelari Mandiri",
    ],
    defaultLayout: {
      ...DEFAULT_LAYOUT_CONFIG,
      templateId: "template-custom",
      heroAlignment: "center",
      heroVariant: "centered",
      themeStyle: "custom",
      customThemePrimary: "#ea580c",
      customThemeAccent: "#f59e0b",
      paperTexture: false,
      showTicker: true,
      tickerText: "CUSTOM EVENT · POWERED BY IVYTICKETING STUDIO ENGINE",
    },
  },
];

export interface HeroSlideConfig {
  id: string;
  tag: string;
  tagColor: string;
  title: string;
  subtitle: string;
  showSubtitle?: boolean;
  bgImage: string;
  primaryCtaText: string;
  primaryCtaLink: string;
  secondaryCtaText?: string;
  secondaryCtaLink?: string;
  stats?: Array<{ label: string; value: string }>;
}

export interface HomepageBrandConfig {
  displayMode: "icon_only" | "text_only" | "both";
  brandName: string;
  tagline: string;
  iconShape: "square" | "rounded" | "hexagon" | "shield";
}

export type NavbarStyleType = "transparent_gradient" | "full_width" | "liquid_glass" | "floating_compact";
export type MobileNavType = "bottom_nav" | "top_bar";

export interface HomepageNavbarConfig {
  style: NavbarStyleType;
  mobileNavType?: MobileNavType;
  opacity: number;
  blur: "none" | "sm" | "md" | "lg";
  showBorder: boolean;
  glassBlur: number;
  glassSaturate: number;
  glassTint: string;
  glassRadius: number;
}

export type ElementCornerStyle = "square" | "rounded" | "pill";

export interface HomepageCornersConfig {
  style: ElementCornerStyle;
}

export type WebsiteThemeMode =
  | "athletic_dark"
  | "midnight_black"
  | "sport_light"
  | "electric_emerald"
  | "speed_cobalt"
  | "custom";

export interface HomepageThemeConfig {
  mode: WebsiteThemeMode;
  bgColor: string;
  surfaceColor: string;
  textColor: string;
  accentColor: string;
}

export interface HomepageHeroConfig {
  mode: "slider" | "single";
  autoplay: boolean;
  intervalSeconds: number;
  overlayDarkness?: number; // 0.1 to 0.9, default 0.35
  statsStyle?: "transparent" | "boxed"; // default "transparent"
  imagePosition?: "top" | "center"; // default "top"
  showTags?: boolean; // default false
  bottomBlur?: boolean; // default true
  slides: HeroSlideConfig[];
}

export interface HomepageFeaturesConfig {
  showSportFilter?: boolean;
}

export interface HomepageConfig {
  brand: HomepageBrandConfig;
  navbar: HomepageNavbarConfig;
  corners?: HomepageCornersConfig;
  hero: HomepageHeroConfig;
  theme: HomepageThemeConfig;
  features?: HomepageFeaturesConfig;
}

export const DEFAULT_HOMEPAGE_CONFIG: HomepageConfig = {
  brand: {
    displayMode: "icon_only",
    brandName: "IvyTicketing",
    tagline: "Sports Registration Engine",
    iconShape: "rounded",
  },
  navbar: {
    style: "transparent_gradient",
    mobileNavType: "bottom_nav",
    opacity: 0.95,
    blur: "md",
    showBorder: true,
    glassBlur: 0,
    glassSaturate: 1,
    glassTint: "rgba(255, 255, 255, 0.255)",
    glassRadius: 48,
  },
  corners: {
    style: "square",
  },
  theme: {
    mode: "athletic_dark",
    bgColor: "#0a1120",
    surfaceColor: "#101a2e",
    textColor: "#f8fafc",
    accentColor: "#ea580c",
  },
  features: {
    showSportFilter: true,
  },
  hero: {
    mode: "slider",
    autoplay: true,
    intervalSeconds: 5,
    overlayDarkness: 0.35,
    statsStyle: "transparent",
    imagePosition: "top",
    showTags: false,
    bottomBlur: true,
    slides: [
      {
        id: "slide-bkk",
        tag: "BANGKOK MIDNIGHT · SUNDAY NIGHT SPECIAL",
        tagColor: "bg-amber-500 text-black font-black",
        title: "Bangkok Midnight Marathon 2026",
        subtitle: "Menembus gemerlap kuil megah Sanam Chai dan landmark Rama VIII di bawah langit malam Bangkok dengan rute steril berlisensi.",
        bgImage: "/images/events/bangkok_marathon.jpg",
        primaryCtaText: "Daftar Bangkok Midnight",
        primaryCtaLink: "/events/bangkok-midnight-marathon-2026",
        secondaryCtaText: "Lihat Detail Rute",
        secondaryCtaLink: "/events/bangkok-midnight-marathon-2026#section-route",
        stats: [
          { label: "Flag-Off", value: "01:00 ICT" },
          { label: "Kategori", value: "42K · 21K · 10K" },
          { label: "Total Peserta", value: "15.000 Pelari" },
        ],
      },
      {
        id: "slide-boston",
        tag: "130TH BOSTON MARATHON · ABBOTT WORLD MAJOR",
        tagColor: "bg-[#fdda24] text-[#002244] font-black",
        title: "130th Boston Marathon 2026",
        subtitle: "Lomba marathon tahunan tertua di dunia. Menaklukkan Hopkinton ke Boylston Street dengan sorak Heartbreak Hill dan standar kualifikasi prestisius.",
        showSubtitle: false,
        bgImage: "/images/events/boston_marathon.jpg",
        primaryCtaText: "Daftar Boston Marathon",
        primaryCtaLink: "/events/boston-marathon-2026",
        secondaryCtaText: "Standar Kualifikasi",
        secondaryCtaLink: "/events/boston-marathon-2026#baa-qualifiers",
        stats: [
          { label: "Tanggal", value: "20 Apr 2026" },
          { label: "Peserta", value: "30.000 Pelari" },
          { label: "Garis Finish", value: "Boylston St" },
        ],
      },
      {
        id: "slide-ironman",
        tag: "",
        tagColor: "",
        title: "IRONMAN 70.3 Triathlon Lombok 2026",
        subtitle: "1.9K Ocean Swim Samudra Hindia, 90K Coastal Highway Cycling, dan 21.1K Sunset Boulevard Run menuju podium dunia.",
        bgImage: "/images/events/ironman_triathlon.jpg",
        primaryCtaText: "Daftar IRONMAN 70.3",
        primaryCtaLink: "/events/ironman-703-triathlon-2026",
        secondaryCtaText: "Jadwal & Logistik",
        secondaryCtaLink: "/events/ironman-703-triathlon-2026#im-schedule",
        stats: [
          { label: "Jarak Total", value: "113.1 KM" },
          { label: "Kona Slots", value: "45 Tiket Dunia" },
          { label: "Status War", value: "Buka Sekarang" },
        ],
      },
      {
        id: "slide-tokyo",
        tag: "ABBOTT WORLD MARATHON MAJOR · JAPAN",
        tagColor: "bg-red-600 text-white font-black",
        title: "Tokyo International Marathon 2026",
        subtitle: "Ajang World Marathon Major pertama di Asia. 38.000 pelari menyusuri Shinjuku, Asakusa, dan Ginza dengan antrean kuota war sistem ballot.",
        bgImage: "/images/events/tokyo_marathon.jpg",
        primaryCtaText: "Daftar Tokyo Marathon",
        primaryCtaLink: "/events/tokyo-international-marathon-2026",
        secondaryCtaText: "Panduan Kuota Ballot",
        secondaryCtaLink: "/events/tokyo-international-marathon-2026#tokyo-quota-table",
        stats: [
          { label: "Kuota Ballot", value: "38.000 Slot" },
          { label: "Cut-Off", value: "07:00:00" },
          { label: "Garis Finish", value: "Tokyo Station" },
        ],
      },
      {
        id: "slide-borobudur",
        tag: "HERITAGE MARATHON · DASA WARSA",
        tagColor: "bg-orange-500 text-black font-black",
        title: "Borobudur Heritage Marathon 2026",
        subtitle: "Menyatu dengan warisan budaya luhur Candi Borobudur dan perbukitan Menoreh diiringi sambutan hangat 15 cheer zones pedesaan.",
        bgImage: "/images/events/borobudur_marathon.jpg",
        primaryCtaText: "Daftar Borobudur Marathon",
        primaryCtaLink: "/events/borobudur-heritage-marathon-2026",
        secondaryCtaText: "Profil Rute 285m",
        secondaryCtaLink: "/events/borobudur-heritage-marathon-2026#section-route",
        stats: [
          { label: "Elevasi", value: "285m Rolling" },
          { label: "Cheer Zones", value: "15 Titik Warga" },
          { label: "Start Fajar", value: "04:30 WIB" },
        ],
      },
      {
        id: "slide-summarecon",
        tag: "URBAN RUN FESTIVAL · NIGHT LIGHTS",
        tagColor: "bg-emerald-500 text-black font-black",
        title: "Summarecon Bandung Run Fest 2026",
        subtitle: "Festival lari kota bernuansa neon kuning cerah dengan lintasan datar terukur, expo olahraga, dan hiburan panggung penutup.",
        bgImage: "/images/events/bandung_runfest.jpg",
        primaryCtaText: "Daftar Bandung Run Fest",
        primaryCtaLink: "/events/summarecon-bandung-runfest-2026",
        secondaryCtaText: "Kategori 10K & 5K",
        secondaryCtaLink: "/events/summarecon-bandung-runfest-2026#section-categories",
        stats: [
          { label: "Karakter", value: "Flat PB Course" },
          { label: "Hadiah", value: "Total 250 Juta" },
          { label: "Status Tiket", value: "Early Bird" },
        ],
      },
      {
        id: "slide-berlin",
        tag: "FASTEST COURSE IN THE WORLD · WORLD RECORD",
        tagColor: "bg-sky-500 text-black font-black",
        title: "BMW Berlin Speed Marathon 2026",
        subtitle: "Lintasan legendaris tempat tumbangnya rekor dunia maraton dengan rute super datar 20m elevasi dan klimaks Gerbang Brandenburg.",
        bgImage: "/images/events/hero_marathon.jpg",
        primaryCtaText: "Daftar Berlin Marathon",
        primaryCtaLink: "/events/berlin-speed-marathon-2026",
        secondaryCtaText: "Detail Rekor Rute",
        secondaryCtaLink: "/events/berlin-speed-marathon-2026#section-route",
        stats: [
          { label: "Elevasi Datar", value: "20m Saja" },
          { label: "Rekor Dunia", value: "2:00:35 WR" },
          { label: "Finish Gate", value: "Gerbang Brandenburg" },
        ],
      },
    ],
  },
};

const HOMEPAGE_CONFIG_STORAGE_KEY = "ivy_platform_homepage_config";

export function getHomepageConfig(): HomepageConfig {
  if (typeof window !== "undefined") {
    try {
      const raw = localStorage.getItem(HOMEPAGE_CONFIG_STORAGE_KEY);
      if (raw) {
        const parsed = JSON.parse(raw);
        return {
          brand: { ...DEFAULT_HOMEPAGE_CONFIG.brand, ...(parsed.brand || {}) },
          navbar: { ...DEFAULT_HOMEPAGE_CONFIG.navbar, ...(parsed.navbar || {}) },
          corners: { ...DEFAULT_HOMEPAGE_CONFIG.corners, ...(parsed.corners || {}) },
          theme: { ...DEFAULT_HOMEPAGE_CONFIG.theme, ...(parsed.theme || {}) },
          features: { ...DEFAULT_HOMEPAGE_CONFIG.features, ...(parsed.features || {}) },
          hero: {
            ...DEFAULT_HOMEPAGE_CONFIG.hero,
            ...(parsed.hero || {}),
            slides:
              parsed.hero?.slides && parsed.hero.slides.length > 0
                ? parsed.hero.slides
                : DEFAULT_HOMEPAGE_CONFIG.hero.slides,
          },
        };
      }
    } catch {
      // fallback
    }
  }
  return DEFAULT_HOMEPAGE_CONFIG;
}

export function saveHomepageConfig(cfg: HomepageConfig): void {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(HOMEPAGE_CONFIG_STORAGE_KEY, JSON.stringify(cfg));
    window.dispatchEvent(
      new CustomEvent("ivy:homepage_config_changed", { detail: cfg })
    );
    // Sinkronisasi latar belakang ke endpoint server bersama
    fetch("/platform-config.json", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }).catch(() => {});
  } catch {
    // quota
  }
}

export function resetHomepageConfig(): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(HOMEPAGE_CONFIG_STORAGE_KEY, JSON.stringify(DEFAULT_HOMEPAGE_CONFIG));
  window.dispatchEvent(
    new CustomEvent("ivy:homepage_config_changed", { detail: DEFAULT_HOMEPAGE_CONFIG })
  );
  fetch("/platform-config.json", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(DEFAULT_HOMEPAGE_CONFIG),
  }).catch(() => {});
}

export async function syncHomepageConfigFromServer(): Promise<HomepageConfig> {
  if (typeof window === "undefined") return DEFAULT_HOMEPAGE_CONFIG;
  try {
    const res = await fetch("/platform-config.json", { cache: "no-store" });
    if (res.ok) {
      const remoteCfg = await res.json();
      const merged: HomepageConfig = {
        brand: { ...DEFAULT_HOMEPAGE_CONFIG.brand, ...(remoteCfg.brand || {}) },
        navbar: { ...DEFAULT_HOMEPAGE_CONFIG.navbar, ...(remoteCfg.navbar || {}) },
        corners: { ...DEFAULT_HOMEPAGE_CONFIG.corners, ...(remoteCfg.corners || {}) },
        theme: { ...DEFAULT_HOMEPAGE_CONFIG.theme, ...(remoteCfg.theme || {}) },
        features: { ...DEFAULT_HOMEPAGE_CONFIG.features, ...(remoteCfg.features || {}) },
        hero: {
          ...DEFAULT_HOMEPAGE_CONFIG.hero,
          ...(remoteCfg.hero || {}),
          slides:
            Array.isArray(remoteCfg.hero?.slides) && remoteCfg.hero.slides.length > 0
              ? remoteCfg.hero.slides
              : DEFAULT_HOMEPAGE_CONFIG.hero.slides,
        },
      };
      localStorage.setItem(HOMEPAGE_CONFIG_STORAGE_KEY, JSON.stringify(merged));
      window.dispatchEvent(
        new CustomEvent("ivy:homepage_config_changed", { detail: merged })
      );
      return merged;
    }
  } catch {
    // offline fallback
  }
  return getHomepageConfig();
}



