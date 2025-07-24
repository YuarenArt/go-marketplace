package app_cmd

import (
	"bufio"
	"context"
	"fmt"
	"github.com/YuarenArt/marketgo/internal/client"
	"github.com/YuarenArt/marketgo/internal/config"
	"github.com/YuarenArt/marketgo/internal/server/services"
	"github.com/YuarenArt/marketgo/pkg/logging"
	"os"
	"strconv"
	"strings"
)

// App представляет консольное приложение для работы с MarketGo API
type App struct {
	client *client.Client
	logger logging.Logger
}

// NewApp создает новое консольное приложение
func NewApp(logger logging.Logger, cfg *config.Config) *App {
	return &App{
		client: client.NewClient(cfg.APIURL, logger),
		logger: logger,
	}
}

// Run запускает приложение в интерактивном режиме
func (a *App) Run() error {
	fmt.Println("Консольное приложение MarketGo. Введите 'help' для списка команд.")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" {
			fmt.Println("Выход из приложения")
			return nil
		}
		if err := a.executeCommand(input); err != nil {
			a.logger.Error("Ошибка выполнения команды", "command", input, "error", err)
			fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		}
	}
}

// executeCommand парсит и выполняет команду
func (a *App) executeCommand(input string) error {
	args := strings.Fields(input)
	if len(args) == 0 {
		return nil
	}

	command := args[0]
	args = args[1:]

	switch command {
	case "help":
		return a.handleHelp()
	case "register":
		return a.handleRegister(args)
	case "login":
		return a.handleLogin(args)
	case "create-ad":
		return a.handleCreateAd(args)
	case "list-ads":
		return a.handleListAds(args)
	default:
		return fmt.Errorf("неизвестная команда: %s. Введите 'help' для списка команд", command)
	}
}

// handleHelp выводит справку по командам
func (a *App) handleHelp() error {
	fmt.Println(`Доступные команды:
  register <login> <password> - Регистрация нового пользователя
  login <login> <password> - Аутентификация пользователя
  create-ad <title> <text> <price> [image_url] - Создание нового объявления
  list-ads [page] [page_size] [sort_by] [sort_order] [min_price] [max_price] - Получение списка объявлений
  exit - Выход из приложения`)
	return nil
}

// handleRegister обрабатывает команду регистрации
func (a *App) handleRegister(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("команда register требует логин и пароль")
	}
	ctx := context.Background()
	input := &services.InputUserInfo{Login: args[0], Password: args[1]}
	user, err := a.client.Register(ctx, input)
	if err != nil {
		return fmt.Errorf("регистрация: %w", err)
	}
	fmt.Printf("Пользователь зарегистрирован: ID=%d, Login=%s\n", user.ID, user.Login)
	a.logger.Info("Регистрация успешна", "login", user.Login, "user_id", user.ID)
	return nil
}

// handleLogin обрабатывает команду входа
func (a *App) handleLogin(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("команда login требует логин и пароль")
	}
	ctx := context.Background()
	input := &services.InputUserInfo{Login: args[0], Password: args[1]}
	if err := a.client.Login(ctx, input); err != nil {
		return fmt.Errorf("вход: %w", err)
	}
	fmt.Printf("Вход выполнен для %s\n", args[0])
	a.logger.Info("Вход успешен", "login", args[0])
	return nil
}

// handleCreateAd обрабатывает команду создания объявления
func (a *App) handleCreateAd(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("команда create-ad требует заголовок, текст и цену")
	}
	price, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return fmt.Errorf("цена должна быть числом: %w", err)
	}
	imageURL := ""
	if len(args) > 3 {
		imageURL = args[3]
	}
	ctx := context.Background()
	req := &services.CreateAdRequest{
		Title:    args[0],
		Text:     args[1],
		Price:    price,
		ImageURL: imageURL,
	}
	ad, err := a.client.PostAdd(ctx, req)
	if err != nil {
		return fmt.Errorf("создание объявления: %w", err)
	}
	fmt.Printf("Объявление создано: ID=%d, Title=%s, Price=%d\n", ad.ID, ad.Title, ad.Price)
	a.logger.Info("Объявление создано", "ad_id", ad.ID, "title", ad.Title)
	return nil
}

// handleListAds обрабатывает команду получения списка объявлений
func (a *App) handleListAds(args []string) error {
	req := services.GetAdsRequest{
		Page:      1,
		PageSize:  10,
		SortBy:    "created_at",
		SortOrder: "DESC",
	}
	if len(args) > 0 {
		page, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("page должен быть числом: %w", err)
		}
		req.Page = page
	}
	if len(args) > 1 {
		pageSize, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("page_size должен быть числом: %w", err)
		}
		req.PageSize = pageSize
	}
	if len(args) > 2 {
		req.SortBy = args[2]
	}
	if len(args) > 3 {
		req.SortOrder = args[3]
	}
	if len(args) > 4 {
		minPrice, err := strconv.ParseInt(args[4], 10, 64)
		if err != nil {
			return fmt.Errorf("min_price должен быть числом: %w", err)
		}
		req.MinPrice = minPrice
	}
	if len(args) > 5 {
		maxPrice, err := strconv.ParseInt(args[5], 10, 64)
		if err != nil {
			return fmt.Errorf("max_price должен быть числом: %w", err)
		}
		req.MaxPrice = maxPrice
	}
	ctx := context.Background()
	ads, err := a.client.GetAds(ctx, req)
	if err != nil {
		return fmt.Errorf("получение объявлений: %w", err)
	}

	if len(ads) == 0 {
		fmt.Println("Объявления не найдены.")
		a.logger.Info("Объявления получены", "page", req.Page, "count", len(ads))
		return nil
	}

	fmt.Printf("Найдено объявлений: %d\n\n", len(ads))
	for i, ad := range ads {
		// Форматируем дату
		createdAt := ad.CreatedAt.Format("2006-01-02 15:04:05")
		// Выводим блок для каждого объявления
		fmt.Println("=============================================================")
		fmt.Printf("Объявление %d\n", ad.ID)
		fmt.Println("-------------------------------------------------------------")
		fmt.Printf("Заголовок:      %s\n", ad.Title)
		fmt.Printf("Текст:          %s\n", ad.Text)
		fmt.Printf("Цена:           %d\n", ad.Price)
		fmt.Printf("URL изображения:%s\n", ad.ImageURL)
		fmt.Printf("Создано:        %s\n", createdAt)
		fmt.Println("=============================================================")
		// Добавляем пустую строку между объявлениями, кроме последнего
		if i < len(ads)-1 {
			fmt.Println()
		}
	}

	a.logger.Info("Объявления получены", "page", req.Page, "count", len(ads))
	return nil
}
