import React, { useState, useEffect, useCallback } from 'react';
import { User, CreateUserRequest } from '../types/api';
import { apiClient } from '../services/api-client';
import { UserCard } from './UserCard';
import { CreateUserModal } from './CreateUserModal';
import styled from 'styled-components';

const Container = styled.div`
  padding: 2rem;
  max-width: 1200px;
  margin: 0 auto;
`;

const Header = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
`;

const Title = styled.h1`
  color: #333;
  margin: 0;
`;

const CreateButton = styled.button`
  background: #007bff;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 1rem;

  &:hover {
    background: #0056b3;
  }

  &:disabled {
    background: #ccc;
    cursor: not-allowed;
  }
`;

const UserGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1rem;
  margin-bottom: 2rem;
`;

const LoadingSpinner = styled.div`
  display: flex;
  justify-content: center;
  align-items: center;
  height: 200px;
  font-size: 1.2rem;
  color: #666;
`;

const ErrorMessage = styled.div`
  background: #f8d7da;
  color: #721c24;
  padding: 1rem;
  border-radius: 4px;
  margin-bottom: 1rem;
`;

const EmptyState = styled.div`
  text-align: center;
  padding: 3rem;
  color: #666;
  
  h3 {
    margin-bottom: 1rem;
  }
`;

interface UserListProps {
  onUserSelect?: (user: User) => void;
  selectedUserId?: number;
}

export const UserList: React.FC<UserListProps> = ({ 
  onUserSelect,
  selectedUserId 
}) => {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState<boolean>(false);
  const [showCreateModal, setShowCreateModal] = useState<boolean>(false);

  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const fetchedUsers = await apiClient.getUsers();
      setUsers(fetchedUsers);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch users');
      console.error('Error fetching users:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleCreateUser = async (userData: CreateUserRequest): Promise<void> => {
    try {
      setCreating(true);
      const newUser = await apiClient.createUser(userData);
      setUsers(prevUsers => [...prevUsers, newUser]);
      setShowCreateModal(false);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to create user';
      setError(errorMessage);
      throw new Error(errorMessage);
    } finally {
      setCreating(false);
    }
  };

  const handleUpdateUser = async (userId: number, userData: Partial<User>): Promise<void> => {
    try {
      const updatedUser = await apiClient.updateUser(userId, userData);
      setUsers(prevUsers => 
        prevUsers.map(user => user.id === userId ? updatedUser : user)
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update user');
      throw err;
    }
  };

  const handleDeleteUser = async (userId: number): Promise<void> => {
    if (!window.confirm('Are you sure you want to delete this user?')) {
      return;
    }

    try {
      await apiClient.deleteUser(userId);
      setUsers(prevUsers => prevUsers.filter(user => user.id !== userId));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete user');
      throw err;
    }
  };

  const handleUserClick = (user: User): void => {
    if (onUserSelect) {
      onUserSelect(user);
    }
  };

  const handleRefresh = (): void => {
    fetchUsers();
  };

  if (loading) {
    return (
      <Container>
        <LoadingSpinner>Loading users...</LoadingSpinner>
      </Container>
    );
  }

  return (
    <Container>
      <Header>
        <Title>Users ({users.length})</Title>
        <div>
          <CreateButton 
            onClick={handleRefresh}
            disabled={loading}
            style={{ marginRight: '1rem' }}
          >
            Refresh
          </CreateButton>
          <CreateButton 
            onClick={() => setShowCreateModal(true)}
            disabled={creating}
          >
            {creating ? 'Creating...' : 'Create User'}
          </CreateButton>
        </div>
      </Header>

      {error && (
        <ErrorMessage>
          {error}
          <button 
            onClick={() => setError(null)}
            style={{ 
              background: 'none', 
              border: 'none', 
              float: 'right',
              cursor: 'pointer',
              color: 'inherit'
            }}
          >
            ×
          </button>
        </ErrorMessage>
      )}

      {users.length === 0 ? (
        <EmptyState>
          <h3>No users found</h3>
          <p>Get started by creating your first user.</p>
          <CreateButton onClick={() => setShowCreateModal(true)}>
            Create First User
          </CreateButton>
        </EmptyState>
      ) : (
        <UserGrid>
          {users.map(user => (
            <UserCard
              key={user.id}
              user={user}
              selected={user.id === selectedUserId}
              onClick={() => handleUserClick(user)}
              onUpdate={(userData) => handleUpdateUser(user.id, userData)}
              onDelete={() => handleDeleteUser(user.id)}
            />
          ))}
        </UserGrid>
      )}

      {showCreateModal && (
        <CreateUserModal
          onSubmit={handleCreateUser}
          onCancel={() => setShowCreateModal(false)}
          loading={creating}
        />
      )}
    </Container>
  );
};

export default UserList;