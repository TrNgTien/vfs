# frozen_string_literal: true

require "json"

# Top-level constant
VERSION = "1.0.0"
MAX_RETRIES = 3

# Top-level method
def top_level_helper(x, y)
  x + y
end

# Module with nested content
module Serializable
  FORMATS = [:json, :xml]

  def serialize
    to_json
  end

  def self.included(base)
    base.extend(ClassMethods)
  end

  module ClassMethods
    def from_json(data)
      new(JSON.parse(data))
    end
  end
end

# Class with inheritance
class Animal
  KINGDOM = "Animalia"

  attr_reader :name, :age
  attr_accessor :color

  def initialize(name, age)
    @name = name
    @age = age
  end

  def speak
    raise NotImplementedError
  end

  def to_s
    "#{name} (#{age})"
  end

  def self.create(name, age)
    new(name, age)
  end

  class << self
    def species_count
      42
    end
  end

  private

  def internal_id
    @_id
  end

  def secret_method
    "hidden"
  end
end

# Subclass
class Dog < Animal
  DEFAULT_TRICKS = ["sit", "stay"]

  def speak
    "Woof!"
  end

  def fetch(item)
    "Fetching #{item}"
  end

  def self.good_boy?(dog)
    true
  end

  protected

  def pack_status
    :alpha
  end
end

# Nested modules and classes
module Services
  module Authentication
    class TokenService
      TOKEN_EXPIRY = 3600

      def generate(user_id)
        "token_#{user_id}"
      end

      def self.validate(token)
        !token.nil?
      end

      private

      def encode(payload)
        Base64.encode64(payload)
      end
    end
  end

  class BaseService
    def call
      raise NotImplementedError
    end

    def self.call(**args)
      new(**args).call
    end
  end
end

# Class with singleton methods
class Configuration
  def self.load(path)
    new(path)
  end

  def self.default
    load("config/default.yml")
  end

  def get(key)
    @store[key]
  end
end

# Private top-level (should not appear)
def _private_helper
  "hidden"
end

# Variable assignment (not a constant, should not appear)
some_var = 42
