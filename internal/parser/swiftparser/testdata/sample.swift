import Foundation
import UIKit

// Top-level function
func topLevelFunc(x: Int) -> String {
    return String(x)
}

// Top-level async throwing function
func fetchData(from url: URL) async throws -> Data {
    return Data()
}

// Top-level properties
var globalVar: Int = 42
let globalLet: String = "hello"

// Type aliases
typealias Completion = (Result<Data, Error>) -> Void
typealias StringDict = [String: Any]

// Class with inheritance
class UserService: BaseService {
    var name: String
    let id: Int

    func getUser(id: Int) -> User {
        return User(name: "", email: "")
    }

    static func create() -> UserService {
        return UserService()
    }

    private func internalMethod() {}

    init(name: String) {
        self.name = name
        self.id = 0
    }
}

// Struct conforming to protocol
struct User: Codable, Hashable {
    let name: String
    let email: String

    func displayName() -> String {
        return name
    }
}

// Protocol with associated type
protocol DataSource {
    associatedtype Item
    func fetchAll() -> [Item]
    var count: Int { get }
}

// Protocol with inheritance
protocol Configurable: AnyObject {
    func configure(with options: [String: Any])
}

// Enum with associated values
enum APIError: Error {
    case notFound
    case unauthorized
    case custom(message: String)
}

// Extension
extension String {
    func trimmed() -> String {
        return trimmingCharacters(in: .whitespaces)
    }

    var isBlank: Bool {
        return isEmpty
    }
}

// Actor
actor DataStore {
    var items: [String] = []

    func add(item: String) {
        items.append(item)
    }
}

// Generic class
class Repository<T: Codable> {
    func findById(id: String) -> T? {
        return nil
    }

    func save(entity: T) {}
}

// Nested types
class Outer {
    struct Inner {
        let value: Int
    }

    enum Direction {
        case up, down, left, right
    }
}

// Private top-level (should not appear)
private func privateHelper() {}
fileprivate class InternalOnly {}
private var _secret: String = "hidden"

// Open class
open class BaseComponent {
    open func render() -> String { return "" }
}
