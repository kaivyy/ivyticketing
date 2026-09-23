import { fetchPublicPaymentChannels } from "./api";

export interface PaymentGatewayConfig {
  code: string;
  name: string;
  type: string;
  isActive: boolean;
  isProduction: boolean;
  merchantCode?: string;
  apiKey?: string;
  secretKey?: string;
  serverKey?: string;
  clientKey?: string;
  callbackUrl?: string;
  description: string;
}

export interface PaymentChannelConfig {
  code: string;
  category: "qris" | "va" | "card";
  method: "qris" | "va" | "ewallet";
  channel: string;
  name: string;
  description: string;
  fee: number;
  feeType: "fixed" | "percent";
  feeBearer: "BUYER" | "ORGANIZER";
  isActive: boolean;
  badge?: string;
  badgeBg: string;
  badgeFg: string;
  logo: string;
}

export interface PlatformPaymentSettings {
  activeGateway: string;
  gateways: PaymentGatewayConfig[];
  channels: PaymentChannelConfig[];
  updatedAt: string;
}

export const DEFAULT_GATEWAYS: PaymentGatewayConfig[] = [
  {
    code: "xendit",
    name: "Xendit Indonesia",
    type: "All-in-One Gateway",
    isActive: true,
    isProduction: false,
    secretKey: "",
    callbackUrl: "https://api.ivyticketing.com/api/v1/payments/webhook/xendit",
    description: "QRIS Dinamis, Virtual Account 6 Bank Nasional, dan E-Wallet resmi.",
  },
  {
    code: "duitku",
    name: "Duitku Payment Gateway",
    type: "High-Volume Direct VA",
    isActive: false,
    isProduction: false,
    merchantCode: "",
    apiKey: "",
    callbackUrl: "https://api.ivyticketing.com/api/v1/payments/webhook/duitku",
    description: "Spesialis Virtual Account Real-Time 24 Jam dan QRIS Merchant Duitku.",
  },
  {
    code: "midtrans",
    name: "Midtrans SNAP",
    type: "Core API & Snap",
    isActive: false,
    isProduction: false,
    serverKey: "",
    clientKey: "",
    callbackUrl: "https://api.ivyticketing.com/api/v1/payments/webhook/midtrans",
    description: "Gateway multi-pembayaran ekosistem GoTo Financial dan Kartu Kredit 3D Secure.",
  },
  {
    code: "simulator",
    name: "Ivy Sandbox Simulator",
    type: "Internal Test Engine",
    isActive: false,
    isProduction: false,
    description: "Engine verifikasi instan internal untuk simulasi dan demonstrasi cepat tanpa kredensial pihak ketiga.",
  },
];

export const DEFAULT_CHANNELS: PaymentChannelConfig[] = [
  // QRIS & E-Wallet
  {
    code: "qris_instant",
    category: "qris",
    method: "qris",
    channel: "qris",
    name: "QRIS Instant",
    description: "BCA Mobile, Livin, GoPay, OVO, DANA, ShopeePay",
    fee: 0,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    badge: "Bebas Biaya",
    logo: "QRIS",
    badgeBg: "bg-red-600",
    badgeFg: "text-white",
  },
  {
    code: "gopay",
    category: "qris",
    method: "ewallet",
    channel: "gopay",
    name: "GoPay Direct",
    description: "Pembayaran instan aplikasi GoPay",
    fee: 1000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    badge: "Instan",
    logo: "GOPAY",
    badgeBg: "bg-sky-500",
    badgeFg: "text-white",
  },
  {
    code: "shopeepay",
    category: "qris",
    method: "ewallet",
    channel: "shopeepay",
    name: "ShopeePay",
    description: "Scan kode atau konfirmasi aplikasi Shopee",
    fee: 1000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    logo: "SPAY",
    badgeBg: "bg-orange-600",
    badgeFg: "text-white",
  },
  {
    code: "dana",
    category: "qris",
    method: "ewallet",
    channel: "dana",
    name: "DANA Wallet",
    description: "Saldo DANA dan Kartu Debit Tersimpan",
    fee: 1000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    logo: "DANA",
    badgeBg: "bg-blue-500",
    badgeFg: "text-white",
  },
  {
    code: "ovo",
    category: "qris",
    method: "ewallet",
    channel: "ovo",
    name: "OVO Cash",
    description: "Konfirmasi notifikasi aplikasi OVO",
    fee: 1000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    logo: "OVO",
    badgeBg: "bg-purple-600",
    badgeFg: "text-white",
  },

  // Virtual Account Bank
  {
    code: "bca_va",
    category: "va",
    method: "va",
    channel: "bca",
    name: "BCA Virtual Account",
    description: "m-BCA, KlikBCA, myBCA, dan ATM BCA",
    fee: 3000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    badge: "Terpopuler",
    logo: "BCA",
    badgeBg: "bg-blue-700",
    badgeFg: "text-white",
  },
  {
    code: "mandiri_va",
    category: "va",
    method: "va",
    channel: "mandiri",
    name: "Mandiri Virtual Account",
    description: "Livin by Mandiri dan ATM Mandiri",
    fee: 3000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    badge: "Otomatis",
    logo: "MANDIRI",
    badgeBg: "bg-amber-600",
    badgeFg: "text-white",
  },
  {
    code: "bni_va",
    category: "va",
    method: "va",
    channel: "bni",
    name: "BNI Virtual Account",
    description: "BNI Mobile Banking dan ATM BNI",
    fee: 3000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    logo: "BNI",
    badgeBg: "bg-teal-700",
    badgeFg: "text-white",
  },
  {
    code: "bri_va",
    category: "va",
    method: "va",
    channel: "bri",
    name: "BRI Virtual Account (BRIVA)",
    description: "BRImo, Internet Banking, dan ATM BRI",
    fee: 3000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    logo: "BRI",
    badgeBg: "bg-blue-600",
    badgeFg: "text-white",
  },
  {
    code: "permata_va",
    category: "va",
    method: "va",
    channel: "permata",
    name: "Permata Virtual Account",
    description: "PermataMobile X dan Jaringan ATM Bersama",
    fee: 3000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    logo: "PERMATA",
    badgeBg: "bg-emerald-600",
    badgeFg: "text-white",
  },
  {
    code: "cimb_va",
    category: "va",
    method: "va",
    channel: "cimb",
    name: "CIMB Niaga Virtual Account",
    description: "OCTO Mobile dan ATM CIMB Niaga",
    fee: 3000,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    logo: "CIMB",
    badgeBg: "bg-red-700",
    badgeFg: "text-white",
  },

  // Kartu Kredit & Debit Online
  {
    code: "card_online",
    category: "card",
    method: "ewallet",
    channel: "card",
    name: "Kartu Kredit / Debit Online",
    description: "Visa, Mastercard, JCB dengan Proteksi 3D Secure",
    fee: 4500,
    feeType: "fixed",
    feeBearer: "BUYER",
    isActive: true,
    badge: "3D Secure",
    logo: "CARD",
    badgeBg: "bg-slate-900",
    badgeFg: "text-white",
  },
];

const STORAGE_KEY = "ivy_platform_payment_config";

export function getDefaultPaymentSettings(): PlatformPaymentSettings {
  return {
    activeGateway: "xendit",
    gateways: DEFAULT_GATEWAYS,
    channels: DEFAULT_CHANNELS,
    updatedAt: new Date().toISOString(),
  };
}

export function getPaymentSettings(): PlatformPaymentSettings {
  if (typeof window === "undefined") {
    return getDefaultPaymentSettings();
  }

  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      const defaults = getDefaultPaymentSettings();
      localStorage.setItem(STORAGE_KEY, JSON.stringify(defaults));
      return defaults;
    }
    const parsed = JSON.parse(raw) as PlatformPaymentSettings;

    // Merge missing gateways or channels if new ones were added
    if (!parsed.gateways || parsed.gateways.length === 0) {
      parsed.gateways = DEFAULT_GATEWAYS;
    }
    if (!parsed.channels || parsed.channels.length === 0) {
      parsed.channels = DEFAULT_CHANNELS;
    }
    return parsed;
  } catch {
    return getDefaultPaymentSettings();
  }
}

export function savePaymentSettings(settings: PlatformPaymentSettings): void {
  if (typeof window === "undefined") return;

  try {
    settings.updatedAt = new Date().toISOString();
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    window.dispatchEvent(
      new CustomEvent("ivy:payment_settings_changed", { detail: settings })
    );
  } catch {
    // quota or private mode
  }
}

/**
 * Returns active channels for participant checkout, optionally taking into account
 * event-specific overrides if configured by organizer.
 */
export function getActiveChannelsForEvent(eventId?: string): PaymentChannelConfig[] {
  const platformSettings = getPaymentSettings();
  let channels = platformSettings.channels.filter((c) => c.isActive);

  if (typeof window !== "undefined" && eventId) {
    try {
      const eventRaw = localStorage.getItem(`ivy_event_payment_config_${eventId}`);
      if (eventRaw) {
        const eventOverrides = JSON.parse(eventRaw) as {
          disabledChannels?: string[];
          customFees?: Record<string, number>;
        };

        if (eventOverrides.disabledChannels?.length) {
          channels = channels.filter(
            (c) => !eventOverrides.disabledChannels?.includes(c.code)
          );
        }

        if (eventOverrides.customFees) {
          channels = channels.map((c) => ({
            ...c,
            fee: eventOverrides.customFees?.[c.code] ?? c.fee,
          }));
        }
      }
    } catch {
      // ignore
    }
  }

  return channels;
}

export async function syncEventPaymentChannelsFromServer(eventId: string): Promise<PaymentChannelConfig[]> {
  try {
    const serverChannels = await fetchPublicPaymentChannels(eventId);
    if (Array.isArray(serverChannels) && serverChannels.length > 0) {
      const disabledCodes = serverChannels
        .filter((sc: any) => sc.is_enabled === false)
        .map((sc: any) => sc.channel_code);

      if (typeof window !== "undefined") {
        try {
          const cfg = { disabledChannels: disabledCodes };
          localStorage.setItem(`ivy_event_payment_config_${eventId}`, JSON.stringify(cfg));
        } catch {}
      }
    }
  } catch (err) {
    console.warn("Could not sync payment channels from server:", err);
  }
  return getActiveChannelsForEvent(eventId);
}
