package com.example

import kotlinx.coroutines.flow.Flow

// Top-level function
fun topLevelFun(x: Int): String {
    return x.toString()
}

// Suspend function
suspend fun fetchData(url: String, timeout: Int = 30): String {
    return ""
}

// Top-level properties
val appName: String = "MyApp"
var retryCount: Int = 3
const val MAX_RETRIES: Int = 5

// Type aliases
typealias StringMap = Map<String, String>
typealias Callback<T> = (T) -> Unit

// Data class
data class User(val name: String, val email: String)

// Sealed class with generics
sealed class Result<T> {
    data class Success<T>(val data: T) : Result<T>()
    data class Error(val message: String) : Result<Nothing>()
}

// Abstract class
abstract class BaseService {
    abstract fun initialize(): Unit
    open fun configure() {}
    private fun internalSetup() {}
}

// Regular class with inheritance
class UserService : BaseService() {
    fun getUser(id: Int): User {
        return User("", "")
    }

    override fun initialize() {}

    companion object {
        fun create(): UserService = UserService()
        const val TAG: String = "UserService"
    }
}

// Interface with generics
interface Repository<T> {
    fun findById(id: String): T?
    fun save(entity: T): Unit
    val count: Int
}

// Object declaration (singleton)
object NetworkModule {
    fun provideClient(): Any = Any()
    val baseUrl: String = "https://api.example.com"
}

// Enum class
enum class Status {
    ACTIVE, INACTIVE, SUSPENDED;
    fun isActive(): Boolean = this == ACTIVE
}

// Annotation class
annotation class Inject

// Generic class with constraint
class GenericRepo<T : Any>(private val items: MutableList<T> = mutableListOf()) {
    fun findAll(): List<T> = items
    fun add(item: T) { items.add(item) }
}

// Inline function
inline fun <reified T> fromJson(json: String): T {
    throw NotImplementedError()
}

// Extension function (top-level)
fun String.capitalize(): String {
    return replaceFirstChar { it.uppercase() }
}

// Class with named companion object
class Config {
    companion object Factory {
        fun default(): Config = Config()
    }
}

// Private top-level (should not appear)
private fun privateHelper(): Unit {}
private class _InternalClass {}
private val _secret: String = "hidden"

// Internal top-level (should not appear)
internal fun internalHelper(): Unit {}
internal class InternalOnlyClass {}
