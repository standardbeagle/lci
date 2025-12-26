package com.example.multiplatform

import kotlinx.coroutines.*
import kotlin.js.JsName

// Phase 5B: Data classes and sealed classes
data class User(val id: Int, val name: String, val email: String?)

sealed class Result<out T> {
    data class Success<T>(val value: T) : Result<T>()
    data class Error(val message: String) : Result<Nothing>()
    object Loading : Result<Nothing>()
}

// Value classes (Kotlin 1.5+)
@JvmInline
value class UserId(val value: Int)

// Companion object and regular class
class UserService {
    companion object {
        const val MAX_USERS = 1000
        
        fun createDefault(): UserService {
            return UserService()
        }
    }
    
    // Properties with custom getters/setters
    var activeUsers: Int = 0
        private set
    
    val isActive: Boolean
        get() = activeUsers > 0
    
    // Suspend functions for coroutines
    suspend fun fetchUser(id: UserId): Result<User> {
        delay(100)
        return Result.Success(User(id.value, "Test User", "test@example.com"))
    }
    
    // Extension function inside class
    fun String.isValidEmail(): Boolean {
        return contains("@")
    }
}

// Interface with default implementation
interface Repository<T> {
    suspend fun save(item: T): Result<T>
    
    fun validate(item: T): Boolean = true // Default implementation
}

// Object declaration (singleton)
object DatabaseConfig {
    const val DATABASE_URL = "localhost:5432"
    
    fun connect(): String {
        return "Connected to $DATABASE_URL"
    }
    
    // Nested object
    object Cache {
        const val TTL_SECONDS = 3600
    }
}

// Enum with properties and functions
enum class UserRole(val permissions: Set<String>) {
    ADMIN(setOf("read", "write", "delete")),
    USER(setOf("read")),
    GUEST(emptySet());
    
    fun hasPermission(permission: String): Boolean {
        return permissions.contains(permission)
    }
}

// Type alias
typealias UserMap = Map<UserId, User>

// Annotations
@Target(AnnotationTarget.FUNCTION)
@Retention(AnnotationRetention.RUNTIME)
annotation class Cached(val ttl: Int = 300)

// Extension functions
fun User.isAdult(): Boolean = id > 18

fun List<User>.findByName(name: String): User? {
    return find { it.name == name }
}

// Extension property
val User.displayName: String
    get() = "$name (ID: $id)"

// Higher-order functions with lambdas
class UserRepository : Repository<User> {
    private val users = mutableMapOf<UserId, User>()
    
    @Cached(ttl = 600)
    override suspend fun save(item: User): Result<User> {
        users[UserId(item.id)] = item
        return Result.Success(item)
    }
    
    fun findUsers(predicate: (User) -> Boolean): List<User> {
        return users.values.filter(predicate)
    }
    
    // Inline function
    inline fun <T> measureTime(block: () -> T): Pair<T, Long> {
        val start = System.currentTimeMillis()
        val result = block()
        val duration = System.currentTimeMillis() - start
        return result to duration
    }
}

// Multiplatform expect/actual declarations would be in separate files
// expect fun platformSpecificFunction(): String

// Coroutine builders and flow
class UserController {
    private val userService = UserService()
    
    fun getUsers(): Flow<User> = flow {
        for (i in 1..10) {
            emit(User(i, "User $i", "user$i@example.com"))
            delay(100)
        }
    }
    
    suspend fun processUsers() = withContext(Dispatchers.IO) {
        val users = mutableListOf<User>()
        getUsers().collect { user ->
            users.add(user)
        }
        users
    }
}