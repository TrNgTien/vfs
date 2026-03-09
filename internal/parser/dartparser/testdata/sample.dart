import 'package:flutter/material.dart';
import 'dart:async';

// Top-level constant
const String appName = 'MyApp';
const int maxRetries = 3;
final double pi = 3.14159;

// Top-level typedef
typedef JsonMap = Map<String, dynamic>;
typedef Compare<T> = int Function(T a, T b);

// Top-level function
void main() {
  runApp(const MyApp());
}

// Top-level async function
Future<String> fetchData(String url, {int timeout = 30}) async {
  return '';
}

// Top-level getter
String get appVersion => '1.0.0';

// Abstract class
abstract class BaseService {
  Future<void> initialize();
  void dispose();
}

// Regular class with extends, implements, with
class UserService extends BaseService with LoggingMixin implements Disposable {
  final String _apiKey;
  final String baseUrl;

  UserService(this._apiKey, {required this.baseUrl});

  UserService.fromConfig(Map<String, dynamic> config)
      : _apiKey = config['apiKey'] as String,
        baseUrl = config['baseUrl'] as String;

  factory UserService.create() {
    return UserService('key', baseUrl: 'https://api.example.com');
  }

  @override
  Future<void> initialize() async {
    // init
  }

  @override
  void dispose() {
    // cleanup
  }

  Future<User> getUser(int id) async {
    return User(id: id, name: 'Test');
  }

  static UserService get instance => _instance;
  static final UserService _instance = UserService.create();

  String get apiKey => _apiKey;

  set timeout(int value) => _timeout = value;

  void _privateMethod() {
    // should not appear
  }
}

// Generic class
class Repository<T extends Model> {
  final List<T> _items = [];

  Future<T?> findById(String id) async {
    return null;
  }

  Future<List<T>> getAll() async {
    return _items;
  }
}

// Sealed class (Dart 3)
sealed class AuthState {}

class Authenticated extends AuthState {
  final User user;
  Authenticated(this.user);
}

class Unauthenticated extends AuthState {}

// Mixin
mixin LoggingMixin on BaseService {
  void log(String message) {
    print(message);
  }
}

// Mixin without on clause
mixin Serializable {
  Map<String, dynamic> toJson();
  String toJsonString() => '';
}

// Extension
extension StringExtension on String {
  String capitalize() {
    if (isEmpty) return this;
    return '${this[0].toUpperCase()}${substring(1)}';
  }

  bool get isBlank => trim().isEmpty;
}

// Enum (enhanced)
enum Status {
  active,
  inactive,
  suspended;

  String get label => name.capitalize();

  bool get isActive => this == Status.active;
}

// Simple enum
enum Color { red, green, blue }

// Flutter Widget
class MyHomePage extends StatefulWidget {
  const MyHomePage({super.key, required this.title});

  final String title;

  @override
  State<MyHomePage> createState() => _MyHomePageState();
}

// Private class (should not appear at top level)
class _MyHomePageState extends State<MyHomePage> {
  int _counter = 0;

  void _incrementCounter() {
    setState(() {
      _counter++;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold();
  }
}

// Extension type (Dart 3.3)
extension type Meters(double value) {
  Meters operator +(Meters other) => Meters(value + other.value);
}

// Class with implements multiple interfaces
class ApiClient implements HttpClient, Configurable {
  @override
  Future<Response> get(String url) async {
    return Response();
  }

  @override
  void configure(Map<String, dynamic> options) {}
}

// Typedef with old syntax
typedef void EventCallback(String event, Map<String, dynamic> data);
