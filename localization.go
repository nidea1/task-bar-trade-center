package main

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const displayLanguageSystem = "system"

type appLocale struct {
	Code string
	Name string
}

var supportedAppLocales = []appLocale{
	{Code: "en-US", Name: "English"},
	{Code: "de-DE", Name: "Deutsch"},
	{Code: "fr-FR", Name: "Français"},
	{Code: "it-IT", Name: "Italiano"},
	{Code: "es-ES", Name: "Español"},
	{Code: "nl-NL", Name: "Nederlands"},
	{Code: "pt-PT", Name: "Português (Portugal)"},
	{Code: "fi-FI", Name: "Suomi"},
	{Code: "ja-JP", Name: "日本語"},
	{Code: "ko-KR", Name: "한국어"},
	{Code: "zh-Hans", Name: "简体中文"},
	{Code: "hi-IN", Name: "हिन्दी"},
	{Code: "id-ID", Name: "Bahasa Indonesia"},
	{Code: "th-TH", Name: "ไทย"},
	{Code: "vi-VN", Name: "Tiếng Việt"},
	{Code: "pt-BR", Name: "Português (Brasil)"},
	{Code: "pl-PL", Name: "Polski"},
	{Code: "tr-TR", Name: "Türkçe"},
}

var (
	displayLanguageMu         sync.RWMutex
	displayLanguage           = "en-US"
	displayLanguagePreference = displayLanguageSystem
	windowsLocaleName         = readWindowsLocaleName
)

var englishMessages = map[string]string{
	"status.starting":                      "Starting…",
	"status.waiting_game":                  "Waiting for TaskBarHero",
	"status.attaching":                     "Connecting to TaskBarHero…",
	"status.ready":                         "Ready",
	"status.admin_required":                "Administrator permission required",
	"status.layout_incompatible":           "Game memory layout needs update",
	"status.initialization_failed":         "Could not finish startup",
	"config.unknown":                       "Not checked yet",
	"config.local_cache":                   "Using cached configuration",
	"config.embedded":                      "Using built-in configuration",
	"config.development":                   "Using local development configuration",
	"config.refreshing":                    "Checking configuration updates…",
	"config.current":                       "Configuration is up to date",
	"config.failed":                        "Could not refresh configuration",
	"update.unknown":                       "Not checked yet",
	"update.checking":                      "Checking for updates…",
	"update.current":                       "Up to date",
	"update.available":                     "Update available: %s",
	"update.failed":                        "Could not check for updates",
	"update.downloading":                   "Downloading update…",
	"menu.status":                          "Status: %s",
	"menu.currency_region":                 "Currency & Region: %s",
	"menu.currency":                        "Currency",
	"menu.language":                        "Language",
	"menu.refresh_cache":                   "Refresh cached prices",
	"menu.clear_cache":                     "Clear cache",
	"menu.compact":                         "Switch to Compact mode",
	"menu.detail":                          "Switch to Detail mode",
	"menu.update_configs":                  "Update configurations",
	"menu.restart_admin":                   "Restart as administrator",
	"menu.install_update":                  "Install update",
	"menu.open_release":                    "Open release page",
	"menu.check_updates":                   "Check for updates…",
	"menu.exit":                            "Exit",
	"menu.created_by":                      "v%s - Created by %s",
	"dialog.already_running":               "Task Bar Trade Center is already running.",
	"dialog.game_closed.title":             "TaskBarHero closed",
	"dialog.game_closed.body":              "TaskBarHero was closed. Do you want to close Task Bar Trade Center too?",
	"dialog.layout_incompatible.title":     "Game memory layout update required",
	"dialog.layout_incompatible.body":      "Task Bar Trade Center could not read the game's memory layout continuously. A TaskBarHero update may have changed it.\n\nThe price HUD has been disabled. Connect to the internet and use Update configurations from the tray menu, or update the application.\n\nDiagnostic log: %s",
	"dialog.restart_failed.title":          "Restart failed",
	"dialog.restart_failed.invalid":        "Task Bar Trade Center could not restart with administrator permission. Please start it manually.",
	"dialog.restart_failed.error":          "Task Bar Trade Center could not restart with administrator permission:\n%v",
	"dialog.update_restart_failed.title":   "Update restart failed",
	"dialog.update_restart_failed.invalid": "Task Bar Trade Center could not restart after the update. Please start it manually.",
	"dialog.update_restart_failed.error":   "Task Bar Trade Center could not restart after the update:\n%v\n\nPlease start it manually.",
	"notification.started":                 "Started",
	"notification.runtime":                 "Status changed — %s",
	"notification.configuration":           "Configuration — %s",
	"notification.update":                  "Update — %s",
	"notification.action_required":         "Right-click the tray icon to act.",
	"notification.admin_restart_failed":    "Could not restart as administrator.",
	"hud.title":                            "TRADE CENTER",
	"hud.market_price":                     "Market Price",
	"hud.suggested":                        "Suggested",
	"hud.last_sold":                        "Last Sold",
	"hud.lowest_sell":                      "Lowest Sell",
	"hud.highest_buy":                      "Highest Buy",
	"hud.weekly_average":                   "Weekly Avg",
	"hud.sale_p75":                         "Sale P75",
	"hud.spread":                           "Spread",
	"hud.trend":                            "Trend",
	"hud.daily_sales":                      "Daily Sales",
	"hud.orders":                           "Orders",
	"hud.open_market":                      "Middle Click: Open Steam Market",
	"hud.loading":                          "Loading price…",
	"value.na":                             "N/A",
	"value.units":                          "units",
	"value.wide":                           "Wide",
	"value.undervalued":                    "Undervalued",
	"value.overvalued":                     "Overvalued",
	"value.verified":                       "Verified",
	"value.estimated":                      "Estimated",
	"value.speculative":                    "Speculative",
	"value.active":                         "Active",
	"value.normal":                         "Normal",
	"value.slow":                           "Slow",
	"time.just_now":                        "just now",
	"time.minutes_ago":                     "%dm ago",
	"time.hours_ago":                       "%dh ago",
	"time.days_ago":                        "%dd ago",
}

func withEnglishFallback(overrides map[string]string) map[string]string {
	result := make(map[string]string, len(englishMessages))
	for key, value := range englishMessages {
		result[key] = value
	}
	for key, value := range overrides {
		result[key] = value
	}
	return result
}

// The shared vocabulary below deliberately keeps product and Steam item names out
// of translations. Those names are market identifiers and must remain canonical.
var localizedMessages = map[string]map[string]string{
	"en-US": englishMessages,
	"de-DE": withEnglishFallback(map[string]string{
		"status.starting": "Wird gestartet…", "status.waiting_game": "Warte auf TaskBarHero", "status.attaching": "Verbindung zu TaskBarHero…", "status.ready": "Bereit", "status.admin_required": "Administratorberechtigung erforderlich", "menu.status": "Status: %s", "menu.show_status": "Status anzeigen", "menu.exit": "Beenden", "window.runtime": "Status", "window.market": "Markt", "window.configuration": "Konfiguration", "window.update": "Update", "window.language": "Sprache", "button.got_it": "Verstanden", "button.restart_admin": "Als Administrator neu starten", "button.install_update": "Update installieren", "hud.suggested": "Empfohlen", "hud.last_sold": "Zuletzt verkauft", "hud.lowest_sell": "Niedrigster Verkauf", "hud.highest_buy": "Höchster Kauf", "hud.weekly_average": "Wochendurchschnitt", "hud.daily_sales": "Tägliche Verkäufe", "hud.open_market": "Mittelklick: Steam-Markt öffnen", "time.just_now": "gerade eben", "time.minutes_ago": "vor %d Min.", "time.hours_ago": "vor %d Std.", "time.days_ago": "vor %d Tagen",
	}),
	"fr-FR": withEnglishFallback(map[string]string{
		"status.starting": "Démarrage…", "status.waiting_game": "En attente de TaskBarHero", "status.attaching": "Connexion à TaskBarHero…", "status.ready": "Prêt", "status.admin_required": "Droits administrateur requis", "menu.status": "État : %s", "menu.show_status": "Afficher l’état", "menu.exit": "Quitter", "window.runtime": "État", "window.market": "Marché", "window.configuration": "Configuration", "window.update": "Mise à jour", "window.language": "Langue", "button.got_it": "Compris", "button.restart_admin": "Redémarrer en administrateur", "button.install_update": "Installer la mise à jour", "hud.suggested": "Suggéré", "hud.last_sold": "Dernière vente", "hud.lowest_sell": "Vente la plus basse", "hud.highest_buy": "Achat le plus élevé", "hud.weekly_average": "Moyenne hebdo.", "hud.daily_sales": "Ventes quotidiennes", "hud.open_market": "Clic central : ouvrir le marché Steam", "time.just_now": "à l’instant", "time.minutes_ago": "il y a %d min", "time.hours_ago": "il y a %d h", "time.days_ago": "il y a %d j",
	}),
	"it-IT": withEnglishFallback(map[string]string{
		"status.starting": "Avvio…", "status.waiting_game": "In attesa di TaskBarHero", "status.attaching": "Connessione a TaskBarHero…", "status.ready": "Pronto", "menu.status": "Stato: %s", "menu.show_status": "Mostra stato", "menu.exit": "Esci", "window.runtime": "Stato", "window.market": "Mercato", "window.configuration": "Configurazione", "window.update": "Aggiornamento", "window.language": "Lingua", "button.got_it": "Ho capito", "button.install_update": "Installa aggiornamento", "hud.suggested": "Suggerito", "hud.last_sold": "Ultima vendita", "hud.lowest_sell": "Vendita minima", "hud.highest_buy": "Acquisto massimo", "hud.weekly_average": "Media sett.", "hud.daily_sales": "Vendite giornaliere", "time.just_now": "ora", "time.minutes_ago": "%d min fa", "time.hours_ago": "%d h fa", "time.days_ago": "%d g fa",
	}),
	"es-ES": withEnglishFallback(map[string]string{
		"status.starting": "Iniciando…", "status.waiting_game": "Esperando a TaskBarHero", "status.attaching": "Conectando con TaskBarHero…", "status.ready": "Listo", "status.admin_required": "Se requiere permiso de administrador", "menu.status": "Estado: %s", "menu.show_status": "Mostrar estado", "menu.exit": "Salir", "window.runtime": "Estado", "window.market": "Mercado", "window.configuration": "Configuración", "window.update": "Actualización", "window.language": "Idioma", "button.got_it": "Entendido", "button.restart_admin": "Reiniciar como administrador", "button.install_update": "Instalar actualización", "hud.suggested": "Sugerido", "hud.last_sold": "Última venta", "hud.lowest_sell": "Venta más baja", "hud.highest_buy": "Compra más alta", "hud.weekly_average": "Promedio semanal", "hud.daily_sales": "Ventas diarias", "time.just_now": "ahora", "time.minutes_ago": "hace %d min", "time.hours_ago": "hace %d h", "time.days_ago": "hace %d d",
	}),
	"nl-NL": withEnglishFallback(map[string]string{
		"status.starting": "Starten…", "status.waiting_game": "Wachten op TaskBarHero", "status.ready": "Gereed", "menu.status": "Status: %s", "menu.show_status": "Status tonen", "menu.exit": "Afsluiten", "window.runtime": "Status", "window.market": "Markt", "window.configuration": "Configuratie", "window.update": "Update", "window.language": "Taal", "button.got_it": "Begrepen", "hud.suggested": "Voorgesteld", "hud.last_sold": "Laatst verkocht", "hud.lowest_sell": "Laagste verkoop", "hud.highest_buy": "Hoogste koop", "hud.weekly_average": "Wekelijks gem.", "hud.daily_sales": "Dagelijkse verkoop", "time.just_now": "zojuist", "time.minutes_ago": "%d min geleden", "time.hours_ago": "%d u geleden", "time.days_ago": "%d d geleden",
	}),
	"pt-PT": withEnglishFallback(map[string]string{
		"status.starting": "A iniciar…", "status.waiting_game": "A aguardar TaskBarHero", "status.ready": "Pronto", "menu.status": "Estado: %s", "menu.show_status": "Mostrar estado", "menu.exit": "Sair", "window.runtime": "Estado", "window.market": "Mercado", "window.configuration": "Configuração", "window.update": "Atualização", "window.language": "Idioma", "button.got_it": "Percebi", "hud.suggested": "Sugerido", "hud.last_sold": "Última venda", "hud.lowest_sell": "Venda mais baixa", "hud.highest_buy": "Compra mais alta", "hud.weekly_average": "Média semanal", "hud.daily_sales": "Vendas diárias", "time.just_now": "agora", "time.minutes_ago": "há %d min", "time.hours_ago": "há %d h", "time.days_ago": "há %d d",
	}),
	"fi-FI": withEnglishFallback(map[string]string{
		"status.starting": "Käynnistetään…", "status.waiting_game": "Odotetaan TaskBarHeroa", "status.ready": "Valmis", "menu.status": "Tila: %s", "menu.show_status": "Näytä tila", "menu.exit": "Lopeta", "window.runtime": "Tila", "window.market": "Markkina", "window.configuration": "Asetukset", "window.update": "Päivitys", "window.language": "Kieli", "button.got_it": "Selvä", "hud.suggested": "Ehdotettu", "hud.last_sold": "Viimeksi myyty", "hud.lowest_sell": "Alin myynti", "hud.highest_buy": "Korkein osto", "hud.weekly_average": "Viikkokeskiarvo", "hud.daily_sales": "Päivämyynti", "time.just_now": "juuri nyt", "time.minutes_ago": "%d min sitten", "time.hours_ago": "%d h sitten", "time.days_ago": "%d pv sitten",
	}),
	"ja-JP": withEnglishFallback(map[string]string{
		"status.starting": "起動中…", "status.waiting_game": "TaskBarHero を待機中", "status.attaching": "TaskBarHero に接続中…", "status.ready": "準備完了", "menu.status": "状態: %s", "menu.show_status": "状態を表示", "menu.exit": "終了", "window.runtime": "状態", "window.market": "マーケット", "window.configuration": "構成", "window.update": "更新", "window.language": "言語", "button.got_it": "了解", "button.install_update": "更新をインストール", "hud.suggested": "推奨", "hud.last_sold": "最終販売", "hud.lowest_sell": "最安販売", "hud.highest_buy": "最高購入", "hud.weekly_average": "週間平均", "hud.daily_sales": "日次販売", "time.just_now": "たった今", "time.minutes_ago": "%d分前", "time.hours_ago": "%d時間前", "time.days_ago": "%d日前",
	}),
	"ko-KR": withEnglishFallback(map[string]string{
		"status.starting": "시작 중…", "status.waiting_game": "TaskBarHero 대기 중", "status.ready": "준비됨", "menu.status": "상태: %s", "menu.show_status": "상태 표시", "menu.exit": "종료", "window.runtime": "상태", "window.market": "마켓", "window.configuration": "구성", "window.update": "업데이트", "window.language": "언어", "button.got_it": "확인", "hud.suggested": "추천", "hud.last_sold": "최근 판매", "hud.lowest_sell": "최저 판매", "hud.highest_buy": "최고 구매", "hud.weekly_average": "주간 평균", "hud.daily_sales": "일일 판매", "time.just_now": "방금", "time.minutes_ago": "%d분 전", "time.hours_ago": "%d시간 전", "time.days_ago": "%d일 전",
	}),
	"zh-Hans": withEnglishFallback(map[string]string{
		"status.starting": "正在启动…", "status.waiting_game": "正在等待 TaskBarHero", "status.ready": "就绪", "menu.status": "状态: %s", "menu.show_status": "显示状态", "menu.exit": "退出", "window.runtime": "状态", "window.market": "市场", "window.configuration": "配置", "window.update": "更新", "window.language": "语言", "button.got_it": "知道了", "hud.suggested": "建议", "hud.last_sold": "最近成交", "hud.lowest_sell": "最低售价", "hud.highest_buy": "最高求购", "hud.weekly_average": "每周平均", "hud.daily_sales": "每日销量", "time.just_now": "刚刚", "time.minutes_ago": "%d分钟前", "time.hours_ago": "%d小时前", "time.days_ago": "%d天前",
	}),
	"hi-IN": withEnglishFallback(map[string]string{
		"status.starting": "शुरू हो रहा है…", "status.waiting_game": "TaskBarHero की प्रतीक्षा में", "status.ready": "तैयार", "menu.status": "स्थिति: %s", "menu.show_status": "स्थिति दिखाएँ", "menu.exit": "बाहर निकलें", "window.runtime": "स्थिति", "window.market": "मार्केट", "window.configuration": "कॉन्फ़िगरेशन", "window.update": "अपडेट", "window.language": "भाषा", "button.got_it": "समझ गया", "hud.suggested": "सुझाया गया", "hud.last_sold": "अंतिम बिक्री", "hud.lowest_sell": "न्यूनतम बिक्री", "hud.highest_buy": "उच्चतम खरीद", "hud.weekly_average": "साप्ताहिक औसत", "hud.daily_sales": "दैनिक बिक्री", "time.just_now": "अभी", "time.minutes_ago": "%d मि. पहले", "time.hours_ago": "%d घं. पहले", "time.days_ago": "%d दि. पहले",
	}),
	"id-ID": withEnglishFallback(map[string]string{
		"status.starting": "Memulai…", "status.waiting_game": "Menunggu TaskBarHero", "status.ready": "Siap", "menu.status": "Status: %s", "menu.show_status": "Tampilkan status", "menu.exit": "Keluar", "window.runtime": "Status", "window.market": "Pasar", "window.configuration": "Konfigurasi", "window.update": "Pembaruan", "window.language": "Bahasa", "button.got_it": "Mengerti", "hud.suggested": "Disarankan", "hud.last_sold": "Terakhir terjual", "hud.lowest_sell": "Jual terendah", "hud.highest_buy": "Beli tertinggi", "hud.weekly_average": "Rata-rata mingguan", "hud.daily_sales": "Penjualan harian", "time.just_now": "baru saja", "time.minutes_ago": "%d mnt lalu", "time.hours_ago": "%d jam lalu", "time.days_ago": "%d hr lalu",
	}),
	"th-TH": withEnglishFallback(map[string]string{
		"status.starting": "กำลังเริ่ม…", "status.waiting_game": "กำลังรอ TaskBarHero", "status.ready": "พร้อม", "menu.status": "สถานะ: %s", "menu.show_status": "แสดงสถานะ", "menu.exit": "ออก", "window.runtime": "สถานะ", "window.market": "ตลาด", "window.configuration": "การตั้งค่า", "window.update": "อัปเดต", "window.language": "ภาษา", "button.got_it": "เข้าใจแล้ว", "hud.suggested": "แนะนำ", "hud.last_sold": "ขายล่าสุด", "hud.lowest_sell": "ราคาขายต่ำสุด", "hud.highest_buy": "ราคาซื้อสูงสุด", "hud.weekly_average": "เฉลี่ยรายสัปดาห์", "hud.daily_sales": "ยอดขายรายวัน", "time.just_now": "เมื่อสักครู่", "time.minutes_ago": "%d นาทีที่แล้ว", "time.hours_ago": "%d ชั่วโมงที่แล้ว", "time.days_ago": "%d วันที่แล้ว",
	}),
	"vi-VN": withEnglishFallback(map[string]string{
		"status.starting": "Đang khởi động…", "status.waiting_game": "Đang chờ TaskBarHero", "status.ready": "Sẵn sàng", "menu.status": "Trạng thái: %s", "menu.show_status": "Hiện trạng thái", "menu.exit": "Thoát", "window.runtime": "Trạng thái", "window.market": "Chợ", "window.configuration": "Cấu hình", "window.update": "Cập nhật", "window.language": "Ngôn ngữ", "button.got_it": "Đã hiểu", "hud.suggested": "Đề xuất", "hud.last_sold": "Bán gần nhất", "hud.lowest_sell": "Bán thấp nhất", "hud.highest_buy": "Mua cao nhất", "hud.weekly_average": "Trung bình tuần", "hud.daily_sales": "Bán hằng ngày", "time.just_now": "vừa xong", "time.minutes_ago": "%d phút trước", "time.hours_ago": "%d giờ trước", "time.days_ago": "%d ngày trước",
	}),
	"pt-BR": withEnglishFallback(map[string]string{
		"status.starting": "Iniciando…", "status.waiting_game": "Aguardando TaskBarHero", "status.ready": "Pronto", "menu.status": "Status: %s", "menu.show_status": "Mostrar status", "menu.exit": "Sair", "window.runtime": "Status", "window.market": "Mercado", "window.configuration": "Configuração", "window.update": "Atualização", "window.language": "Idioma", "button.got_it": "Entendi", "hud.suggested": "Sugerido", "hud.last_sold": "Última venda", "hud.lowest_sell": "Menor venda", "hud.highest_buy": "Maior compra", "hud.weekly_average": "Média semanal", "hud.daily_sales": "Vendas diárias", "time.just_now": "agora", "time.minutes_ago": "há %d min", "time.hours_ago": "há %d h", "time.days_ago": "há %d d",
	}),
	"pl-PL": withEnglishFallback(map[string]string{
		"status.starting": "Uruchamianie…", "status.waiting_game": "Oczekiwanie na TaskBarHero", "status.ready": "Gotowe", "menu.status": "Stan: %s", "menu.show_status": "Pokaż stan", "menu.exit": "Zakończ", "window.runtime": "Stan", "window.market": "Rynek", "window.configuration": "Konfiguracja", "window.update": "Aktualizacja", "window.language": "Język", "button.got_it": "Rozumiem", "hud.suggested": "Sugerowana", "hud.last_sold": "Ostatnia sprzedaż", "hud.lowest_sell": "Najniższa sprzedaż", "hud.highest_buy": "Najwyższy kupno", "hud.weekly_average": "Śr. tygodniowa", "hud.daily_sales": "Dzienne sprzedaże", "time.just_now": "przed chwilą", "time.minutes_ago": "%d min temu", "time.hours_ago": "%d godz. temu", "time.days_ago": "%d dni temu",
	}),
	"tr-TR": withEnglishFallback(map[string]string{
		"status.starting": "Başlatılıyor…", "status.waiting_game": "TaskBarHero bekleniyor", "status.attaching": "TaskBarHero’a bağlanılıyor…", "status.ready": "Hazır", "status.admin_required": "Yönetici izni gerekli", "status.layout_incompatible": "Oyun bellek düzeni güncellenmeli", "status.initialization_failed": "Başlatma tamamlanamadı", "config.local_cache": "Önbellekteki yapılandırma kullanılıyor", "config.embedded": "Yerleşik yapılandırma kullanılıyor", "config.refreshing": "Yapılandırma güncellemeleri denetleniyor…", "config.current": "Yapılandırma güncel", "config.failed": "Yapılandırma yenilenemedi", "update.checking": "Güncellemeler denetleniyor…", "update.current": "Güncel", "update.available": "Güncelleme var: %s", "update.failed": "Güncellemeler denetlenemedi", "update.downloading": "Güncelleme indiriliyor…", "menu.status": "Durum: %s", "menu.show_status": "Durumu göster", "menu.currency_region": "Para Birimi ve Bölge: %s", "menu.currency": "Para birimi", "menu.refresh_cache": "Önbellekteki fiyatları yenile", "menu.clear_cache": "Önbelleği temizle", "menu.compact": "Kompakt moda geç", "menu.detail": "Ayrıntı moduna geç", "menu.update_configs": "Yapılandırmaları güncelle", "menu.check_updates": "Güncellemeleri denetle…", "menu.exit": "Çıkış", "window.status_title": "Task Bar Trade Center durumu", "window.runtime": "Durum", "window.market": "Piyasa", "window.configuration": "Yapılandırma", "window.update": "Güncelleme", "window.language": "Dil", "window.welcome_title": "Task Bar Trade Center bildirim alanında çalışır.", "window.welcome_body": "Bu pencereyi istediğiniz zaman tray simgesinden açabilirsiniz. Uygulama TaskBarHero’yu bekler ve otomatik bağlanır.", "button.got_it": "Anladım", "button.restart_admin": "Yönetici olarak yeniden başlat", "button.install_update": "Güncellemeyi yükle", "button.open_release": "Sürüm sayfasını aç", "dialog.already_running": "Task Bar Trade Center zaten çalışıyor.", "dialog.game_closed.title": "TaskBarHero kapandı", "dialog.game_closed.body": "TaskBarHero kapandı. Task Bar Trade Center da kapatılsın mı?", "dialog.update_available": "Yeni sürüm durum penceresinden yüklenmeye hazır.", "dialog.layout_incompatible.title": "Oyun bellek düzeni güncellemesi gerekli", "dialog.layout_incompatible.body": "Task Bar Trade Center oyunun bellek düzenini sürekli okuyamadı. Bir TaskBarHero güncellemesi düzeni değiştirmiş olabilir.\n\nFiyat HUD’ı devre dışı bırakıldı. İnternete bağlanın ve tray menüsünden Yapılandırmaları güncelle’yi kullanın ya da uygulamayı güncelleyin.\n\nTanılama günlüğü: %s", "hud.market_price": "Piyasa Fiyatı", "hud.suggested": "Önerilen", "hud.last_sold": "Son Satış", "hud.lowest_sell": "En Düşük Satış", "hud.highest_buy": "En Yüksek Alış", "hud.weekly_average": "Haftalık Ort.", "hud.sale_p75": "Satış P75", "hud.spread": "Makas", "hud.trend": "Eğilim", "hud.daily_sales": "Günlük Satış", "hud.orders": "Emirler", "hud.open_market": "Orta Tık: Steam Market’i Aç", "hud.loading": "Fiyat yükleniyor…", "value.units": "adet", "value.wide": "Geniş", "value.undervalued": "Düşük Değerli", "value.overvalued": "Yüksek Değerli", "value.verified": "Doğrulanmış", "value.estimated": "Tahmini", "value.speculative": "Spekülatif", "value.active": "Aktif", "value.normal": "Normal", "value.slow": "Yavaş", "time.just_now": "az önce", "time.minutes_ago": "%d dk önce", "time.hours_ago": "%d sa önce", "time.days_ago": "%d gün önce",
	}),
}

func init() {
	for _, catalog := range localizedMessages {
		for _, key := range []string{
			"menu.show_status",
			"window.status_title", "window.runtime", "window.market", "window.configuration", "window.update", "window.language", "window.welcome_title", "window.welcome_body",
			"button.got_it", "button.restart_admin", "button.install_update", "button.open_release",
			"dialog.update_available",
		} {
			delete(catalog, key)
		}
	}
	localizedMessages["tr-TR"]["menu.language"] = "Dil"
	localizedMessages["tr-TR"]["menu.restart_admin"] = "Yönetici olarak yeniden başlat"
	localizedMessages["tr-TR"]["menu.install_update"] = "Güncellemeyi yükle"
	localizedMessages["tr-TR"]["menu.open_release"] = "Sürüm sayfasını aç"
	localizedMessages["tr-TR"]["notification.started"] = "Başlatıldı"
	localizedMessages["tr-TR"]["notification.runtime"] = "Durum değişti — %s"
	localizedMessages["tr-TR"]["notification.configuration"] = "Yapılandırma — %s"
	localizedMessages["tr-TR"]["notification.update"] = "Güncelleme — %s"
	localizedMessages["tr-TR"]["notification.action_required"] = "İşlem için tray simgesine sağ tıklayın."
	localizedMessages["tr-TR"]["notification.admin_restart_failed"] = "Yönetici olarak yeniden başlatılamadı."
}

func tr(key string, args ...any) string {
	displayLanguageMu.RLock()
	locale := displayLanguage
	displayLanguageMu.RUnlock()
	text := localizedMessages[locale][key]
	if text == "" {
		text = englishMessages[key]
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

func currentDisplayLanguage() string {
	displayLanguageMu.RLock()
	defer displayLanguageMu.RUnlock()
	return displayLanguage
}

func currentDisplayLanguagePreference() string {
	displayLanguageMu.RLock()
	defer displayLanguageMu.RUnlock()
	return displayLanguagePreference
}

func displayLanguageName(code string) string {
	for _, locale := range supportedAppLocales {
		if locale.Code == code {
			return locale.Name
		}
	}
	return "English"
}

func selectDisplayLanguage(preference string) bool {
	if preference == "" {
		preference = displayLanguageSystem
	}
	resolved := resolveDisplayLanguage(preference)
	displayLanguageMu.Lock()
	changed := displayLanguage != resolved || displayLanguagePreference != preference
	displayLanguage = resolved
	displayLanguagePreference = preference
	displayLanguageMu.Unlock()
	if !changed {
		return false
	}
	saveSettingsToDisk()
	requestStatusRefresh()
	if ShowOverlay.Load() {
		redrawOverlay()
	}
	return true
}

func applyDisplayLanguagePreference(preference string) {
	if preference == "" {
		preference = displayLanguageSystem
	}
	displayLanguageMu.Lock()
	displayLanguagePreference = preference
	displayLanguage = resolveDisplayLanguage(preference)
	displayLanguageMu.Unlock()
}

func resolveDisplayLanguage(preference string) string {
	if preference != "" && preference != displayLanguageSystem {
		if supportedDisplayLanguage(preference) {
			return preference
		}
	}
	return mapSystemLocale(windowsLocaleName())
}

func supportedDisplayLanguage(code string) bool {
	for _, locale := range supportedAppLocales {
		if locale.Code == code {
			return true
		}
	}
	return false
}

func mapSystemLocale(locale string) string {
	normalized := strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	for _, supported := range supportedAppLocales {
		if strings.EqualFold(normalized, supported.Code) {
			return supported.Code
		}
	}
	if strings.HasPrefix(normalized, "pt-br") {
		return "pt-BR"
	}
	if strings.HasPrefix(normalized, "zh") {
		return "zh-Hans"
	}
	for _, supported := range supportedAppLocales {
		if strings.HasPrefix(normalized, strings.ToLower(supported.Code[:2])) {
			return supported.Code
		}
	}
	return "en-US"
}

func readWindowsLocaleName() string {
	var locale [85]uint16
	if result, _, _ := procGetUserDefaultLocaleName.Call(uintptr(unsafePointer(&locale[0])), uintptr(len(locale))); result == 0 {
		return "en-US"
	}
	return syscall.UTF16ToString(locale[:])
}

func localizedSemanticValue(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "undervalued", "overvalued", "verified", "estimated", "speculative", "active", "normal", "slow":
		return tr("value." + normalized)
	default:
		return value
	}
}

// unsafePointer keeps the Windows call isolated from the rest of the locale code.
func unsafePointer(value *uint16) uintptr { return uintptr(unsafe.Pointer(value)) }
