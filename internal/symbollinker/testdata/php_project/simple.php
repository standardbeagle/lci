<?php
declare(strict_types=1);

namespace App\Examples;

use App\Services\UserService;
use App\Models\User as UserModel;
use App\Interfaces\{ServiceInterface, RepositoryInterface};
use function helper_function;
use const GLOBAL_CONSTANT;

require_once 'config.php';
require_once __DIR__ . '/helpers.php';

/**
 * A simple PHP class for testing symbol extraction
 */
class SimpleClass implements ServiceInterface
{
    private string $privateProperty;
    protected int $protectedProperty;
    public UserModel $publicProperty;
    
    public const PUBLIC_CONSTANT = 'public_value';
    private const PRIVATE_CONSTANT = 'private_value';
    
    public function __construct(private readonly UserService $userService)
    {
        $this->privateProperty = 'initialized';
    }
    
    public function publicMethod(string $param1, int $param2 = 10): string
    {
        $localVar = 'test';
        return $this->privateMethod($localVar);
    }
    
    private function privateMethod(string $input): string
    {
        return strtoupper($input);
    }
    
    protected function protectedMethod(): void
    {
        // Protected method implementation
    }
    
    public static function staticMethod(): array
    {
        return [];
    }
    
    abstract public function abstractMethod(): bool;
}

abstract class AbstractBase
{
    abstract public function mustImplement(): void;
    
    public function concreteMethod(): string
    {
        return 'base implementation';
    }
}

trait ExampleTrait
{
    private string $traitProperty;
    
    public function traitMethod(): string
    {
        return 'from trait';
    }
    
    abstract public function traitAbstractMethod(): void;
}

interface RepositoryInterface
{
    public function save(UserModel $model): bool;
    public function findById(int $id): ?UserModel;
}

final class FinalClass extends AbstractBase
{
    use ExampleTrait;
    
    public function mustImplement(): void
    {
        // Implementation
    }
    
    public function traitAbstractMethod(): void
    {
        // Implementation
    }
}

// Global functions
function global_function(string $param): string
{
    global $globalVar;
    static $staticVar = 0;
    
    $staticVar++;
    return $param . '_processed';
}

function variadic_function(...$params): array
{
    return $params;
}

// Global constants
define('DEFINED_CONSTANT', 'defined_value');
const GLOBAL_CONST = 'global_value';

// Global variables
$globalVar = 'global';
$anotherGlobal = new SimpleClass(new UserService());

// Anonymous functions
$closure = function(int $x, int $y) use ($globalVar): int {
    return $x + $y;
};

$arrowFunction = fn($x) => $x * 2;

// Enums (PHP 8.1+)
enum Status: string
{
    case PENDING = 'pending';
    case ACTIVE = 'active';
    case INACTIVE = 'inactive';
    
    public function getLabel(): string
    {
        return match($this) {
            Status::PENDING => 'Pending',
            Status::ACTIVE => 'Active',
            Status::INACTIVE => 'Inactive',
        };
    }
}

// Match expression with anonymous class
$handler = new class implements RepositoryInterface {
    public function save(UserModel $model): bool
    {
        return true;
    }
    
    public function findById(int $id): ?UserModel
    {
        return null;
    }
};