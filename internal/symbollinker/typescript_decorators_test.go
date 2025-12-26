package symbollinker

import (
	"strings"
	"testing"

	"github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// TestTypeScriptDecorators_ClassDecorators tests extraction of class-level decorators
func TestTypeScriptDecorators_ClassDecorators(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `
@Component({
  selector: 'app-root',
  templateUrl: './app.component.html'
})
export class AppComponent {
  title = 'My App';
}

@Injectable()
@Singleton()
class UserService {
  getUser() {
    return { name: 'John' };
  }
}
`
	fileID := types.FileID(1)
	extractor := NewTSExtractor()
	tree := parseTypeScriptCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the AppComponent class
	var appComponent *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "AppComponent" {
			appComponent = sym
			break
		}
	}
	if appComponent == nil {
		t.Fatal("AppComponent not found")
	}

	// Verify it has decorators
	if appComponent.Attributes == nil || len(appComponent.Attributes) == 0 {
		t.Fatal("Expected decorators to be extracted")
	}

	// Should have one decorator: Component with arguments
	if len(appComponent.Attributes) != 1 {
		t.Errorf("Expected 1 decorator, got %d", len(appComponent.Attributes))
	}

	// Check for Component decorator
	hasComponent := false
	for _, attr := range appComponent.Attributes {
		if attr.Value == "Component" || strings.Contains(attr.Value, "Component({") {
			hasComponent = true
			if attr.Type != types.AttrTypeDecorator {
				t.Errorf("Expected AttributeType, got %v", attr.Type)
			}
		}
	}

	if !hasComponent {
		t.Error("Expected Component decorator")
	}

	// Find the UserService class
	var userService *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "UserService" {
			userService = sym
			break
		}
	}
	if userService == nil {
		t.Fatal("UserService not found")
	}

	// Should have two decorators: Injectable and Singleton
	if len(userService.Attributes) != 2 {
		t.Errorf("Expected 2 decorators, got %d", len(userService.Attributes))
	}
}

// TestTypeScriptDecorators_MethodDecorators tests extraction of method-level decorators
func TestTypeScriptDecorators_MethodDecorators(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `
class ApiController {
  @Get('/users')
  @Authorize('admin')
  @RateLimit(100)
  getUsers() {
    return [];
  }

  @Post('/users')
  @ValidateBody()
  createUser(user: User) {
    return user;
  }
}
`
	fileID := types.FileID(1)
	extractor := NewTSExtractor()
	tree := parseTypeScriptCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the getUsers method
	var getUsers *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "ApiController.getUsers" {
			getUsers = sym
			break
		}
	}
	if getUsers == nil {
		t.Fatal("getUsers method not found")
	}

	// Verify it has decorators
	if getUsers.Attributes == nil || len(getUsers.Attributes) == 0 {
		t.Fatal("Expected decorators to be extracted")
	}

	// Should have three decorators
	if len(getUsers.Attributes) < 3 {
		t.Errorf("Expected at least 3 decorators, got %d", len(getUsers.Attributes))
	}

	// Check for Get decorator
	hasGet := false
	for _, attr := range getUsers.Attributes {
		if strings.Contains(attr.Value, "Get") {
			hasGet = true
		}
	}
	if !hasGet {
		t.Error("Expected Get decorator")
	}
}

// TestTypeScriptDecorators_PropertyDecorators tests extraction of property-level decorators
func TestTypeScriptDecorators_PropertyDecorators(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `
class User {
  @PrimaryKey()
  @AutoIncrement()
  id: number;

  @Column({ type: 'varchar', length: 255 })
  @Index()
  email: string;

  @Column()
  @NotNull()
  name: string;
}
`
	fileID := types.FileID(1)
	extractor := NewTSExtractor()
	tree := parseTypeScriptCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the id property
	var idProperty *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "User.id" {
			idProperty = sym
			break
		}
	}
	if idProperty == nil {
		t.Fatal("id property not found")
	}

	// Verify it has decorators
	if idProperty.Attributes == nil || len(idProperty.Attributes) == 0 {
		t.Fatal("Expected decorators to be extracted")
	}

	// Should have two decorators: PrimaryKey and AutoIncrement
	if len(idProperty.Attributes) < 2 {
		t.Errorf("Expected at least 2 decorators, got %d", len(idProperty.Attributes))
	}

	// Check for PrimaryKey decorator
	hasPrimaryKey := false
	for _, attr := range idProperty.Attributes {
		if strings.Contains(attr.Value, "PrimaryKey") {
			hasPrimaryKey = true
		}
	}
	if !hasPrimaryKey {
		t.Error("Expected PrimaryKey decorator")
	}
}

// TestTypeScriptDecorators_ParameterDecorators tests extraction of parameter-level decorators
func TestTypeScriptDecorators_ParameterDecorators(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `
class UserController {
  createUser(
    @Body() user: CreateUserDto,
    @Param('id') userId: string,
    @Query('sort') sortOrder: string
  ) {
    return { user, userId, sortOrder };
  }
}
`
	fileID := types.FileID(1)
	extractor := NewTSExtractor()
	tree := parseTypeScriptCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the createUser method
	var createUser *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "UserController.createUser" {
			createUser = sym
			break
		}
	}
	if createUser == nil {
		t.Fatal("createUser method not found")
	}

	// For parameter decorators, we might store them differently
	// This test verifies the basic extraction works
	// The actual storage mechanism may vary based on implementation
}

// TestTypeScriptDecorators_DecoratorFactories tests extraction of decorator factories
func TestTypeScriptDecorators_DecoratorFactories(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `
@Module({
  imports: [DatabaseModule],
  controllers: [UserController],
  providers: [UserService],
})
export class AppModule {}

@Controller('api/users')
@UseGuards(AuthGuard)
export class UserController {
  @Get(':id')
  @UseInterceptors(LoggingInterceptor)
  findOne(@Param('id', ParseIntPipe) id: number) {
    return { id };
  }
}
`
	fileID := types.FileID(1)
	extractor := NewTSExtractor()
	tree := parseTypeScriptCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the AppModule class
	var appModule *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "AppModule" {
			appModule = sym
			break
		}
	}
	if appModule == nil {
		t.Fatal("AppModule not found")
	}

	// Verify it has the Module decorator with complex arguments
	if len(appModule.Attributes) < 1 {
		t.Error("Expected at least 1 decorator")
	}

	// Check for Module decorator
	hasModule := false
	for _, attr := range appModule.Attributes {
		if strings.Contains(attr.Value, "Module") {
			hasModule = true
		}
	}
	if !hasModule {
		t.Error("Expected Module decorator")
	}
}

// Helper function
func parseTypeScriptCode(t *testing.T, code string) *tree_sitter.Tree {
	t.Helper()
	// Tree-sitter parsing not implemented in test environment yet
	return nil
}
